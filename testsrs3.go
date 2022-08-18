package main

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/amdf/ipk"
	"github.com/amdf/ixxatvci3/candev"
)

var gVersion, gYear, gProgramName string
var gStatusString binding.String

func main() {
	gVersion, gYear = "1.0.0", "2022 г." // todo править при изменениях
	gProgramName = "Электронная имитация параметров"
	var err error

	// Инит
	getTomlUPP()
	err = initIPK()
	if err != nil {
		fmt.Printf("Ошибка инициализации ИПК: %v\n", err)
		err = errors.New("Ошибка инициализации ИПК")
	}

	err = can25.Init(0x1F, 0x16)
	if err != nil {
		fmt.Printf("Ошибка инициализации CAN: %v\n", err)
		err = errors.New("Ошибка инициализации CAN")
	}
	can25.Run()
	defer can25.Stop()

	// Форма
	a := app.New()
	w := a.NewWindow(gProgramName)
	// w.Resize(fyne.NewSize(800, 600))
	w.CenterOnScreen()
	w.SetMaster()

	menu := fyne.NewMainMenu(
		fyne.NewMenu("Файл",
			// a quit item will be appended to our first menu
			fyne.NewMenuItem("Тема", func() { changeTheme(a) }),
			// fyne.NewMenuItem("Выход", func() { a.Quit() }),
		),

		fyne.NewMenu("Справка",
			fyne.NewMenuItem("Посмотреть справку", func() { aboutHelp() }),
			// fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("О программе", func() { abautProgramm() }),
		),
	)
	w.SetMainMenu(menu)

	go func() { // простите
		time.Sleep(1 * time.Second)
		for _, item := range menu.Items[0].Items {
			if item.Label == "Quit" {
				item.Label = "Выход"
			}
		}
	}()

	// одна общая строка для вывода ошибок, подсказок
	var style fyne.TextStyle
	style.Monospace = true
	gStatusLabel := widget.NewLabel("Статус")
	gStatusLabel.TextStyle = style
	gStatusString = binding.NewString()
	gStatusLabel.Bind(gStatusString)
	if err != nil {
		gStatusString.Set(fmt.Sprintf("%s", err.Error()))
	}

	// Элементы
	boxSpeed := speed()
	boxOutput := outputSignals()
	box1 := container.NewHSplit(boxSpeed, boxOutput)

	boxInput := inputSignals()
	box2 := container.NewVSplit(box1, boxInput)

	top := top()
	box3 := container.NewVSplit(top, box2)

	boxCAN := getListCAN()
	box4 := container.NewHSplit(box3, boxCAN)

	box := container.NewVSplit(box4, gStatusLabel)

	w.SetContent(box)
	w.ShowAndRun()
}

var gCurrentTheme bool

func changeTheme(a fyne.App) {
	gCurrentTheme = !gCurrentTheme

	if gCurrentTheme {
		a.Settings().SetTheme(theme.DarkTheme())
	} else {
		a.Settings().SetTheme(theme.LightTheme())
	}
}

func aboutHelp() {
	err := exec.Command("cmd", "/C", ".\\help\\index.html").Run()
	if err != nil {
		fmt.Println("Ошибка открытия файла справки")
	}
}

func abautProgramm() {
	w := fyne.CurrentApp().NewWindow("О программе") // CurrentApp!
	w.Resize(fyne.NewSize(400, 150))
	w.SetFixedSize(true)
	w.CenterOnScreen()

	img := canvas.NewImageFromURI(storage.NewFileURI("ind.png"))
	img.Resize(fyne.NewSize(66, 90)) //без изменений
	img.Move(fyne.NewPos(10, 10))

	l0 := widget.NewLabel(gProgramName)
	l0.Move(fyne.NewPos(80, 10))
	l1 := widget.NewLabel(fmt.Sprintf("Версия %s", gVersion))
	l1.Move(fyne.NewPos(80, 40))
	l2 := widget.NewLabel(fmt.Sprintf("© ПАО «Электромеханика», %s", gYear))
	l2.Move(fyne.NewPos(80, 70))

	box := container.NewWithoutLayout(img, l0, l1, l2)

	// w.SetContent(fyne.NewContainerWithLayout(layout.NewCenterLayout(), box))
	w.SetContent(box)
	w.Show() // ShowAndRun -- panic!
}

func getTitle(str string) *widget.Label {
	var style fyne.TextStyle
	style.Bold = true

	return widget.NewLabelWithStyle(str, fyne.TextAlignCenter, style)
}

//---------------------------------------------------------------------------//
// 								Данные CAN
//---------------------------------------------------------------------------//
var mu sync.Mutex
var gDataCAN = make(map[uint32][8]byte)
var gBuErrors []int

func getDataCAN() map[uint32][8]byte {
	mapDataCAN := make(map[uint32][8]byte)

	mu.Lock()
	mapDataCAN = gDataCAN
	mu.Unlock()

	return mapDataCAN
}

func safeError(data [8]byte) {
	var code int

	if data[0] == 1 { // код ошибки установлен
		code = (int(data[2]) << 8) | int(data[1]) // todo проверить на диапазон?
	}
	for _, val := range gBuErrors {
		if val == code {
			return
		}
	}
	gBuErrors = append(gBuErrors, code)
}

func getListCAN() fyne.CanvasObject {

	requestCAN()
	getCAN()

	var data []string

	list := widget.NewList(
		func() int {
			return len(data)
		},
		func() fyne.CanvasObject {
			var style fyne.TextStyle
			style.Monospace = true
			temp := widget.NewLabelWithStyle("temp", fyne.TextAlignLeading, style)
			return temp
			// return widget.NewLabel("template")
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(data[id])
		})

	list.OnSelected = func(id widget.ListItemID) {
		if strings.HasPrefix(data[id], "H") {
			sCodeError := strings.TrimPrefix(data[id], "H")
			// найти в toml файле, который сначала нужно сделать
			// описание в строку статуса
			sErrorDescription := getErrorDescription(sCodeError)
			gStatusString.Set(fmt.Sprintf("H%s: %s", sCodeError, sErrorDescription))
		} else {
			gStatusString.Set("")
		}
	}

	// mapDataCAN := make(map[uint32][8]byte)
	// обновление данных
	go func() {
		for {
			data = nil // todo выводить только то, что есть в CAN? без второй сорости и тд?
			// mu.Lock()
			// mapDataCAN = gDataCAN
			// mu.Unlock()
			mapDataCAN := getDataCAN()

			t := byteToTimeBU(mapDataCAN[idTimeBU]) // todo concurrent map read and map write
			data = append(data, fmt.Sprintf("Время БУ: %s", t.Format("02.01.2006 15:04")))
			data = append(data, " ")

			data = append(data, fmt.Sprintf("%-22s %.1f", "Скорость 1 (км/ч):", byteToSpeed(mapDataCAN[idSpeed1])))
			data = append(data, fmt.Sprintf("%-22s %.1f", "Скорость 2 (км/ч):", byteToSpeed(mapDataCAN[idSpeed2])))
			tm, tc, gr := byteToPressure(mapDataCAN[idPressure])
			data = append(data, fmt.Sprintf("%-22s %.1f", "Давление ТМ (кг/см²):", tm))
			data = append(data, fmt.Sprintf("%-22s %.1f", "Давление ТС (кг/см²):", tc))
			data = append(data, fmt.Sprintf("%-22s %.1f", "Давление ГР (кг/см²):", gr))
			u := byteDistance(mapDataCAN[idDistance])
			data = append(data, fmt.Sprintf("%-22s %d", "Дистанция (м):", u)) // число на 22
			_, str := byteToALS(mapDataCAN[idALS])
			data = append(data, " ")

			data = append(data, fmt.Sprintf("%-16s %s", "АЛС:", str)) // текст на 16
			_, _, _, str = byteToCodeIF(mapDataCAN[idCodeIF])
			data = append(data, fmt.Sprintf("%-16s %s", "Сигнал ИФ:", str))
			canmsg := mapDataCAN[idBin]
			if (canmsg[1] & 0x01) == 0x01 {
				str = "установлено"
			} else {
				str = "сброшено"
			}
			data = append(data, fmt.Sprintf("%-16s %s", "Движение вперёд:", str))
			if (canmsg[1] & 0x02) == 0x02 {
				str = "установлено"
			} else {
				str = "сброшено"
			}
			data = append(data, fmt.Sprintf("%-16s %s", "Движение назад:", str))
			if (canmsg[1] & 0x10) == 0x10 {
				str = "установлен"
			} else {
				str = "сброшен"
			}
			data = append(data, fmt.Sprintf("%-16s %s", "Сигнал Тяга:", str))

			str = byteToDigitalIndicator(mapDataCAN[idDigitalInd])
			data = append(data, fmt.Sprintf("%-16s %s", "Осн. инд.:", str))
			str = byteToAddIndicator(mapDataCAN[idAddInd])
			data = append(data, fmt.Sprintf("%-16s %s", "Доп. инд.:", str))

			buErrors := append(gBuErrors)
			gBuErrors = nil
			if len(buErrors) > 0 {
				data = append(data, " ")
				data = append(data, "Ошибки:")
				// var temp []int
				// for errorcode := range gBuErrors {
				// temp = append(temp, int(errorcode))
				// }
				sort.Ints(buErrors)
				for _, x := range buErrors {
					if x != 0 {
						data = append(data, fmt.Sprintf("H%d", x))
					}
				}
			}

			list.Refresh()
			time.Sleep(2 * time.Second)
		}
	}()

	boxList := container.New(layout.NewGridWrapLayout(fyne.NewSize(290, 695)), list)
	box := container.NewVBox(getTitle("Данные CAN:"), boxList)

	return box
}

func requestCAN() {
	go func() {
		for {
			var msg candev.Message
			msg.ID = idErrors
			msg.Rtr = true
			can25.Send(msg)
			time.Sleep(time.Millisecond * 100)

			msg.ID = idStatusBI
			msg.Rtr = false
			msg.Len = 4
			msg.Data = [8]byte{0xFF, 0, 0, 0x01}
			can25.Send(msg)
			time.Sleep(time.Millisecond * 100)

			// msg.ID = idBI
			// msg.Len = 4
			// msg.Data = [8]byte{0x04, 0xFF, 0, 0}
			// can25.Send(msg)

			time.Sleep(1 * time.Second / 2)
		}
	}()
}

func getCAN() {

	// timeCheckDone := make(chan int) // признак того что результат готов

	go func() {
		stop := false
		ch, _ := can25.GetMsgChannelCopy()

		for !stop {
			// получение данных

			select {
			case msg, ok := <-ch:
				if !ok { //при закрытом канале
					stop = true
				} else {
					mu.Lock()
					if msg.ID == idErrors {
						safeError(msg.Data)
					} else if msg.ID == idBI && msg.Data[0] == 0x01 {
						gDataCAN[idDigitalInd] = msg.Data
					} else if msg.ID == idBI && msg.Data[0] == 0x02 {
						gDataCAN[idAddInd] = msg.Data
					} else {
						gDataCAN[msg.ID] = msg.Data
					}
					mu.Unlock()

					// defer can25.CloseMsgChannelCopy(idx)

				}
			default:
			}
			runtime.Gosched()

			// обновление данных
			// time.Sleep(200 * time.Millisecond)
		}
		// timeCheckDone <- 1
	}()
}

//---------------------------------------------------------------------------//
// 						ИНТЕРФЕЙС: ФАС, ФЧС
//---------------------------------------------------------------------------//

func newSpecialEntry(initValue string) (e *numericalEntry) {
	e = newNumericalEntry()
	e.Entry.Wrapping = fyne.TextTruncate
	e.Entry.TextStyle.Monospace = true
	// e.Entry.SetPlaceHolder(placeHolder)
	e.Entry.SetText(initValue)
	return e
}

// Скорость, дистанция, давление
func speed() fyne.CanvasObject {
	var err error

	// ------------------------- box 1 ----------------------------

	separately := binding.NewBool() // cовместное-раздельное управление
	direction1 := uint8(ipk.MotionOnward)
	direction2 := uint8(ipk.MotionOnward)
	speed1, speed2, accel1, accel2 := float64(0), float64(0), float64(0), float64(0)
	dummy := widget.NewLabel("")

	// обработка скорости
	entrySpeed1 := newSpecialEntry("0.0")
	entrySpeed2 := newSpecialEntry("0.0")

	entrySpeed1.Entry.OnChanged = func(str string) {
		str = strings.ReplaceAll(str, ",", ".")
		if speed1, err = strconv.ParseFloat(str, 64); err != nil {
			fmt.Printf("Ошибка перевода строки в число (скорость 1)\n")
			gStatusString.Set("Ошибка в поле ввода «Скорость 1»")
			return
		}
		if sep, _ := separately.Get(); !sep { // !не раздельное управление
			speed2 = speed1                // тоже в переменную
			entrySpeed2.Entry.SetText(str) // и в поле второго канала скорости
		}
	}
	entrySpeed1.Entry.OnSubmitted = func(str string) { // todo если пусто устанавливать ноль?
		if err = sp.SetSpeed(speed1, speed2); err != nil {
			fmt.Printf("Ошибка установки скорости")
			gStatusString.Set("Ошибка установки скорости")
		} else {
			gStatusString.Set(" ")
		}
		entrySpeed1.Entry.SetText(fmt.Sprintf("%.1f", speed1))
		entrySpeed2.Entry.SetText(fmt.Sprintf("%.1f", speed2))
		fmt.Printf("Скорость: %.1f %.1f км/ч (%v)\n", speed1, speed2, err)
	}

	entrySpeed2.Entry.OnChanged = func(str string) {
		str = strings.ReplaceAll(str, ",", ".")
		if speed2, err = strconv.ParseFloat(str, 64); err != nil {
			fmt.Printf("Ошибка перевода строки в число (скорость 2)\n")
			gStatusString.Set("Ошибка в поле ввода «Скорость 2»")
			return
		}
		if sep, _ := separately.Get(); !sep {
			speed1 = speed2
			entrySpeed1.Entry.SetText(str)
		}
	}
	entrySpeed2.Entry.OnSubmitted = func(str string) {
		if err = sp.SetSpeed(speed1, speed2); err != nil {
			fmt.Printf("Ошибка установки скорости")
			gStatusString.Set("Ошибка установки скорости")
		} else {
			gStatusString.Set(" ")
		}
		entrySpeed1.Entry.SetText(fmt.Sprintf("%.1f", speed1))
		entrySpeed2.Entry.SetText(fmt.Sprintf("%.1f", speed2))
		fmt.Printf("Скорость: %.1f %.1f км/ч (%v)\n", speed1, speed2, err)
	}

	// обработка ускорения
	entryAccel1 := newSpecialEntry("0.00")
	entryAccel2 := newSpecialEntry("0.00")

	entryAccel1.Entry.OnChanged = func(str string) {
		str = strings.ReplaceAll(str, ",", ".")
		if accel1, err = strconv.ParseFloat(str, 64); err != nil {
			fmt.Printf("Ошибка перевода строки в число (ускорение 1)\n")
			gStatusString.Set("Ошибка в поле ввода «Ускорение 1»")
			return
		}
		if sep, _ := separately.Get(); !sep {
			accel2 = accel1
			entryAccel2.Entry.SetText(str)
		}
	}
	entryAccel1.Entry.OnSubmitted = func(str string) {
		if err = sp.SetAcceleration(accel1*100, accel2*100); err != nil {
			fmt.Printf("Ошибка установки ускорения\n")
			gStatusString.Set("Ошибка установки ускорения")
		} else {
			gStatusString.Set(" ")
		}
		entryAccel1.Entry.SetText(fmt.Sprintf("%.2f", accel1))
		entryAccel2.Entry.SetText(fmt.Sprintf("%.2f", accel2))
		fmt.Printf("Ускорение: %.1f %.1f м/с2 (%v)\n", accel1, accel2, err)
	}

	entryAccel2.Entry.OnChanged = func(str string) {
		str = strings.ReplaceAll(str, ",", ".")
		if accel2, err = strconv.ParseFloat(str, 64); err != nil {
			fmt.Printf("Ошибка перевода строки в число (ускорение 2)\n")
			gStatusString.Set("Ошибка в поле ввода «Ускорение 2»")
			return
		}
		if sep, _ := separately.Get(); !sep {
			accel1 = accel2
			entryAccel1.Entry.SetText(str)
		}
	}
	entryAccel2.Entry.OnSubmitted = func(str string) {

		if err = sp.SetAcceleration(accel1*100, accel2*100); err != nil {
			fmt.Printf("Ошибка установки ускорения\n")
			gStatusString.Set("Ошибка установки ускорения")
		} else {
			gStatusString.Set(" ")
		}
		entryAccel1.Entry.SetText(fmt.Sprintf("%.2f", accel1))
		entryAccel2.Entry.SetText(fmt.Sprintf("%.2f", accel2))
		fmt.Printf("Ускорение: %.1f %.1f м/с2 (%v)\n", accel1, accel2, err)
	}

	// обработка направления
	directionChoice := []string{"Вперёд", "Назад"}
	var selectDirection1, selectDirection2 *widget.Select

	selectDirection1 = widget.NewSelect(directionChoice, func(s string) {
		sep, _ := separately.Get()
		if s == "Вперёд" {
			direction1 = ipk.MotionOnward
			if !sep && selectDirection2.SelectedIndex() != 0 {
				selectDirection2.SetSelectedIndex(0)
			}
		} else {
			direction1 = ipk.MotionBackwards
			if !sep && selectDirection1.SelectedIndex() != 1 {
				selectDirection2.SetSelectedIndex(1)
			}
		}
		sp.SetMotion(direction1) // todo должно быть два напревления
	})
	selectDirection2 = widget.NewSelect(directionChoice, func(s string) {
		sep, _ := separately.Get()
		if s == "Вперёд" {
			direction2 = ipk.MotionOnward
			if !sep && selectDirection2.SelectedIndex() != 0 {
				selectDirection2.SetSelectedIndex(0)
			}
		} else {
			direction2 = ipk.MotionBackwards
			if !sep && selectDirection1.SelectedIndex() != 1 {
				selectDirection1.SetSelectedIndex(1)
			}
		}
		sp.SetMotion(direction2)
	})
	selectDirection1.SetSelectedIndex(0) //"Вперёд"
	selectDirection2.SetSelectedIndex(0) //"Вперёд"

	separatlyCheck := widget.NewCheckWithData("Раздельное управление", separately)

	box1 := container.NewGridWithColumns(
		3,
		dummy, widget.NewLabel("Канал 1"), widget.NewLabel("Канал 2"),
		widget.NewLabel("Скорость (км/ч):"), entrySpeed1, entrySpeed2,
		widget.NewLabel("Ускорение (м/с²):"), entryAccel1, entryAccel2,
		widget.NewLabel("Направление:"), selectDirection1, selectDirection2,
	)

	boxSpeed := container.NewVBox(getTitle("Имитация движения:"), box1, separatlyCheck)

	// ------------------------- box 2 ----------------------------

	startDistanceCheck := false
	distance := 0
	startDistance := uint32(0)

	// обработка пути
	entryMileage := newSpecialEntry("20000")
	entryMileage.Entry.OnChanged = func(str string) {
		distance, err = strconv.Atoi(str)
		if err != nil {
			// distance = 0
			// entryMileage.Entry.SetText("0")
			fmt.Printf("Ошибка перевода строки в число (путь)\n")
		}
	}
	buttonMileage := widget.NewButton("Пуск", func() { // todo стоп
		if 0 == distance {
			return
		}
		startDistanceCheck = true
		startDistance, _, err = sp.GetWay() // todo по двум каналам!
		if err != nil {
			fmt.Printf("Ошибка: не получено значение пути с ИПК\n")
		}

		err = sp.SetLimitWay(uint32(distance))
		if err != nil {
			fmt.Printf("Ошибка установки пути\n")
		}
		// sp.SetMotion(ipk.MotionBackwards)
		// sp.SetSpeed(fScaleLimit, fScaleLimit) // скорость должны установить сами в поле ввода скорости
		fmt.Printf("Путь: %d м (%v)\n", distance, err)
	})
	labelMileage := widget.NewLabel("0")

	box2 := container.NewGridWithColumns(
		3,
		widget.NewLabel("Дистанция:"), entryMileage, buttonMileage,
		widget.NewLabel("Текущая:"), labelMileage,
	)
	boxMileage := container.NewVBox(getTitle("Имитация пути (м):"), box2)

	go func() {
		for {
			if startDistanceCheck {
				m, _, err := sp.GetWay()
				if err != nil {
					fmt.Printf("Не получено значение пути с ИПК\n")
				}
				labelMileage.SetText(fmt.Sprintf("%d", startDistance-m))
			}
			time.Sleep(time.Second)
		}
	}()

	// ------------------------- box 3 ----------------------------

	var press1, press2, press3 float64
	// границы
	limit1, limit2, limit3 := 10., gDevice.PressureLimit, 10.

	// обработка давления
	entryPress1 := newSpecialEntry("0.0")
	entryPress1.Entry.OnChanged = func(str string) {
		str = strings.ReplaceAll(str, ",", ".")
		press1, err = strconv.ParseFloat(str, 64)
		if err != nil {
			fmt.Printf("Ошибка перевода строки в число (давление 1)\n")
			gStatusString.Set("Ошибка в поле ввода «Давление 1»")
			return
		}
		if press1 > limit1 {
			gStatusString.Set(fmt.Sprintf("Давление 1: максимум %.0f кгс/см2", limit1))
		}
	}
	entryPress1.Entry.OnSubmitted = func(str string) {
		if err = channel1.Set(press1); err != nil {
			fmt.Printf("Ошибка установки давления 1\n")
			gStatusString.Set("Ошибка установки давления 1")
		} else {
			gStatusString.Set(" ")
		}
		fmt.Printf("Давление 1: %.1f кгс/см2 (%v)\n", press1, err)
		entryPress1.Entry.SetText(fmt.Sprintf("%.1f", press1))
	}

	entryPress2 := newSpecialEntry("0.0")
	entryPress2.Entry.OnChanged = func(str string) {
		str = strings.ReplaceAll(str, ",", ".")
		press2, err = strconv.ParseFloat(str, 64)
		if err != nil {
			fmt.Printf("Ошибка перевода строки в число (давление 2)\n")
			gStatusString.Set("Ошибка в поле ввода «Давление 2»")
			return
		}
		if press2 > limit2 {
			gStatusString.Set(fmt.Sprintf("Давление 2: максимум %.0f кгс/см2", limit2))
		}
	}
	entryPress2.Entry.OnSubmitted = func(str string) {
		if err = channel2.Set(press2); err != nil {
			fmt.Printf("Ошибка установки давления 2\n")
			gStatusString.Set("Ошибка установки давления 2")
		} else {
			gStatusString.Set(" ")
		}
		fmt.Printf("Давление 2: %.1f кгс/см2 (%v)\n", press2, err)
		entryPress2.Entry.SetText(fmt.Sprintf("%.1f", press2))
	}

	entryPress3 := newSpecialEntry("0.0")
	entryPress3.Entry.OnChanged = func(str string) {
		str = strings.ReplaceAll(str, ",", ".")
		press3, err = strconv.ParseFloat(str, 64)
		if err != nil {
			fmt.Printf("Ошибка перевода строки в число (давление 3)\n")
			gStatusString.Set("Ошибка в поле ввода «Давление 3»")
			return
		}
		if press3 > limit3 {
			gStatusString.Set(fmt.Sprintf("Давление 3: максимум %.0f кгс/см2", limit3))
		}
	}
	entryPress3.Entry.OnSubmitted = func(str string) {
		if err = channel3.Set(press3); err != nil {
			fmt.Printf("Ошибка установки давления 3\n")
		} else {
			gStatusString.Set(" ")
		}
		fmt.Printf("Давление 3: %.1f кгс/см2 (%v)\n", press3, err)
		entryPress3.Entry.SetText(fmt.Sprintf("%.1f", press3))
	}

	box3 := container.NewGridWithColumns(
		3,
		widget.NewLabel("Канал 1:"), widget.NewLabel("Канал 2:"), widget.NewLabel("Канал 3:"),
		entryPress1, entryPress2, entryPress3,
	)
	boxPress := container.NewVBox(getTitle("Имитация давления (кгс/см²):"), box3)

	// -------------------------extra box 3 ----------------------------

	// обработка доп. параметры
	sp.Init(fcs, uint32(gDevice.NumberTeeth), uint32(gDevice.BandageDiameter1)) // предустановка

	entryDiameter := newNumericalEntry()
	entryDiameter.Entry.Wrapping = fyne.TextWrapOff
	entryDiameter.Entry.TextStyle.Monospace = true
	entryDiameter.Entry.SetText(fmt.Sprintf("%d", gDevice.BandageDiameter1))
	entryDiameter.Entry.OnChanged = func(str string) {
		if val, err := strconv.Atoi(str); err != nil {
			fmt.Printf("Ошибка перевода строки в число (диаметр бандажа)\n")
			gStatusString.Set("Ошибка в поле ввода «Дистанция»")
		} else {
			gDevice.BandageDiameter1 = uint32(val)
			sp.Init(fcs, uint32(gDevice.NumberTeeth), uint32(gDevice.BandageDiameter1))
		}
	}

	entryNumberTeeth := newNumericalEntry()
	entryNumberTeeth.Entry.Wrapping = fyne.TextWrapOff
	entryNumberTeeth.Entry.TextStyle.Monospace = true
	entryNumberTeeth.Entry.SetText(fmt.Sprintf("%d", gDevice.NumberTeeth))
	entryNumberTeeth.Entry.OnChanged = func(str string) {
		if val, err := strconv.Atoi(str); err != nil {
			fmt.Printf("Ошибка перевода строки в число (количество зубьев)\n")
			gStatusString.Set("Ошибка в поле ввода «Кол-во зубьев»")
		} else {
			gDevice.NumberTeeth = uint32(val)
			sp.Init(fcs, uint32(gDevice.NumberTeeth), uint32(gDevice.BandageDiameter1))
		}
	}
	extbox := container.NewHBox(widget.NewLabel("Число зубьев:     "), entryNumberTeeth, widget.NewLabel("Диаметр (мм):  "), entryDiameter)
	extParam := container.NewVBox(getTitle("Параметры имитатора:"), extbox)

	boxAll := container.NewVBox(boxSpeed, boxMileage, boxPress, extParam, dummy)
	box := container.NewHBox(dummy, boxAll, dummy)

	return box
}

//---------------------------------------------------------------------------//
// 						ИНТЕРФЕЙС: ФДС сигналы
//---------------------------------------------------------------------------//

// коды РЦ (Сигналы ИФ) +
// Вых.БУ: 50В, 10В
func outputSignals() fyne.CanvasObject {
	var err error

	code := []string{"Нет",
		"КЖ 1.6",
		"Ж 1.6",
		"З 1.6",
		"КЖ 1.9",
		"Ж 1.9",
		"З 1.9",
	}
	radio := widget.NewRadioGroup(code, func(s string) {
		fds.SetIF(ipk.IFEnable)
		switch s {
		case "Нет":
			err = fds.SetIF(ipk.IFDisable) // todo эти разобрать
		case "КЖ 1.6":
			err = fds.SetIF(ipk.IFRedYellow16)
		case "Ж 1.6":
			err = fds.SetIF(ipk.IFYellow16)
		case "З 1.6":
			err = fds.SetIF(ipk.IFGreen16)
		case "КЖ 1.9":
			err = fds.SetIF(ipk.IFRedYellow19)
		case "Ж 1.9":
			err = fds.SetIF(ipk.IFYellow19)
		case "З 1.9":
			err = fds.SetIF(ipk.IFGreen19)
		default:
			fmt.Println("Ошибка выбора кода РЦ")
		}
		fmt.Printf("Код РЦ: %s (%v)\n", s, err)
	})
	radio.SetSelected("Нет")
	// radio.Horizontal = true
	boxCode := container.NewVBox(getTitle("Коды РЦ:      "), radio)

	// 10V
	checkG := widget.NewCheck("З", func(on bool) {
		if on {
			err = fds.Set10V(0, true)
		} else {
			err = fds.Set10V(0, false)
		}
		fmt.Printf("Двоичные выходы 50В: 0=%v З (%v)\n", on, err)
	})
	checkY := widget.NewCheck("Ж", func(on bool) {
		if on {
			err = fds.Set10V(1, true)
		} else {
			err = fds.Set10V(1, false)
		}
		fmt.Printf("Двоичные выходы 50В: 1=%v Ж(%v)\n", on, err)
	})
	checkRY := widget.NewCheck("КЖ", func(on bool) {
		if on {
			err = fds.Set10V(2, true)
		} else {
			err = fds.Set10V(2, false)
		}
		fmt.Printf("Двоичные выходы 50В: 2=%v КЖ (%v)\n", on, err)
	})
	checkR := widget.NewCheck("К", func(on bool) {
		if on {
			err = fds.Set10V(3, true)
		} else {
			err = fds.Set10V(3, false)
		}
		fmt.Printf("Двоичные выходы 50В: 3=%v К (%v)\n", on, err)
	})
	checkW := widget.NewCheck("Б", func(on bool) {
		if on {
			err = fds.Set10V(4, true)
		} else {
			err = fds.Set10V(4, false)
		}
		fmt.Printf("Двоичные выходы 50В: 4=%v Б (%v)\n", on, err)
	})
	checkEPK1 := widget.NewCheck("ЭПК1", func(on bool) {
		if on {
			err = fds.Set10V(5, true)
		} else {
			err = fds.Set10V(5, false)
		}
		fmt.Printf("Двоичные выходы 50В: 5=%v ЭПК1 (%v)\n", on, err)
	})
	checkIF := widget.NewCheck("ИФ", func(on bool) {
		if on {
			err = fds.Set10V(6, true)
		} else {
			err = fds.Set10V(6, false)
		}
		fmt.Printf("Двоичные выходы 50В: 6=%v ИФ (%v)\n", on, err)
	})
	checkTracktion := widget.NewCheck("Тяга", func(on bool) {
		if on {
			err = fds.Set10V(7, true)
		} else {
			err = fds.Set10V(7, false)
		}
		fmt.Printf("Двоичные выходы 50В: 7=%v Тяга (%v)\n", on, err)
	})
	boxOut50V := container.NewVBox(checkG, checkY, checkRY, checkR, checkW, checkEPK1, checkIF, checkTracktion)
	// 50V
	checkLP := widget.NewCheck("ЛП", func(on bool) {
		if on {
			err = fds.Set50V(1, true)
		} else {
			err = fds.Set50V(1, false)
		}
		fmt.Printf("Двоичные выходы 10В: 1=%v ЛП (%v)\n", on, err)
	})
	checkButtonUhod := widget.NewCheck("кн. Уход", func(on bool) {
		if on {
			err = fds.Set50V(3, true)
		} else {
			err = fds.Set50V(3, false)
		}
		fmt.Printf("Двоичные выходы 10В: 3=%v кн. Уход (%v)\n", on, err)
	})
	checkEPK := widget.NewCheck("ЭПК", func(on bool) {
		if on {
			err = fds.Set50V(5, true)
		} else {
			err = fds.Set50V(5, false)
		}
		fmt.Printf("Двоичные выходы 10В: 5=%v ЭПК (%v)\n", on, err)
	})
	checkPowerBU := widget.NewCheck("Пит.БУ", func(on bool) {
		if on {
			err = fds.Set50V(7, true)
		} else {
			err = fds.Set50V(7, false)
		}
		fmt.Printf("Двоичные выходы 10В: 7=%v Пит.БУ (%v)\n", on, err)
	})
	checkKeyEPK := widget.NewCheck("Ключ ЭПК ", func(on bool) {
		if on {
			err = fds.Set50V(9, true)
		} else {
			err = fds.Set50V(9, false)
		}
		fmt.Printf("Двоичные выходы 10В: 9=%v Ключ ЭПК (%v)\n", on, err)
	})
	boxOut10V := container.NewVBox(checkLP, checkButtonUhod, checkEPK, checkPowerBU, checkKeyEPK)

	boxOut := container.NewVBox(getTitle("Вых.БУ:"), boxOut10V, boxOut50V)
	box := container.NewHBox(boxOut, boxCode)

	return box
}

// Уставки, входы БУС = считать
func inputSignals() fyne.CanvasObject {

	check1 := widget.NewCheck("1", nil)
	check20 := widget.NewCheck("20", nil)
	checkY := widget.NewCheck(gUPP[14].Value, nil)  // 80 V(ж)
	checkRY := widget.NewCheck(gUPP[15].Value, nil) // 60 V(кж)
	checkU := widget.NewCheck(gUPP[16].Value, nil)  // 30 V(упр)
	boxRelay := container.NewHBox(check1, check20, checkY, checkRY, checkU)

	// labelBUS := widget.NewLabel("Входы БУС:")
	checkPSS2 := widget.NewCheck("ПСС2", nil)
	checkUhod2 := widget.NewCheck("Уход 2", nil)
	checkPowerEPK := widget.NewCheck("Пит.ЭПК", nil)
	checkPB2 := widget.NewCheck("РБ2", nil)
	checkEVM := widget.NewCheck("ЭВМ", nil)
	boxBUS := container.NewHBox(checkPSS2, checkUhod2, checkPowerEPK, checkPB2, checkEVM)

	box := container.NewHBox(boxRelay, boxBUS)

	go func() {
		for {
			bin, err := fas.UintGetBinaryInput()
			if err != nil {
				// fmt.Printf("Ошибка получения двоичного сигнала\n") отладка
			}

			if bin&0x100 == 0x100 {
				check1.SetChecked(true)
			} else {
				check1.SetChecked(false)
			}
			if bin&0x200 == 0x200 {
				check20.SetChecked(true)
			} else {
				check20.SetChecked(false)
			}
			if bin&0x400 == 0x400 {
				checkY.SetChecked(true)
			} else {
				checkY.SetChecked(false)
			}
			if bin&0x800 == 0x800 {
				checkRY.SetChecked(true)
			} else {
				checkRY.SetChecked(false)
			}
			if bin&0x1000 == 0x1000 {
				checkU.SetChecked(true)
			} else {
				checkU.SetChecked(false)
			}
			pss2, _ := fas.GetBinaryInputVal(0) // ПСС2
			if pss2 {
				checkPSS2.SetChecked(true)
			} else {
				checkPSS2.SetChecked(false)
			}
			uhod2, _ := fas.GetBinaryInputVal(1) // УХОД
			if uhod2 {
				checkUhod2.SetChecked(true)
			} else {
				checkUhod2.SetChecked(false)
			}
			epk, _ := fas.GetBinaryInputVal(2) // Пит. ЭПК
			if epk {
				checkPowerEPK.SetChecked(true)
			} else {
				checkPowerEPK.SetChecked(false)
			}
			rb2, _ := fas.GetBinaryInputVal(3) // РБC
			if rb2 {
				checkPB2.SetChecked(true)
			} else {
				checkPB2.SetChecked(false)
			}
			emv, _ := fas.GetBinaryInputVal(4) // ЭМВ
			if emv {
				checkEVM.SetChecked(true)
			} else {
				checkEVM.SetChecked(false)
			}

			time.Sleep(time.Second)
		}
	}()

	return container.NewVBox(getTitle("Реле превышения уставок:"), box)
}

//---------------------------------------------------------------------------//
// 								ИНТЕРФЕЙС: верх
//---------------------------------------------------------------------------//

func top() fyne.CanvasObject {

	deviceChoice := []string{"БУ-3П", "БУ-3ПА", "БУ-3ПВ", "БУ-4"}
	var selectDevice *widget.Select
	selectDevice = widget.NewSelect(deviceChoice, func(s string) {
		gDevice.NameBU = s
	})
	selectDevice.SetSelectedIndex(2)

	checkPower := widget.NewCheck("Питание КПД", func(on bool) {
		powerBU(on)
		gDevice.Power = on
	})
	checkPower.SetChecked(true) // питание включается при старте? todo

	checkTurt := widget.NewCheck("Режим обслуживания", func(on bool) {
		turt(on)
	})

	buttonUPP := widget.NewButton("  УПП  ", func() {
		showFormUPP()
	})

	box := container.New(layout.NewHBoxLayout(), selectDevice, checkPower, checkTurt, layout.NewSpacer(), buttonUPP)

	// box := container.NewHBox(selectDevice, checkPower, checkTurt, buttonUPP)
	return box // container.New(layout.NewGridWrapLayout(fyne.NewSize(400, 35)), box)
}

//---------------------------------------------------------------------------//

// todo вот после обновления УПП нужно еще и на форме значения обновить? 42 1350

func showFormUPP() {
	var paramEntry = make(map[int]*widget.Entry) // todo добавить в gUPP?
	statusLabel := widget.NewLabel(" ")

	// переход в режим обслуживания
	// defer обратненько

	w := fyne.CurrentApp().NewWindow("УПП") // CurrentApp!
	w.Resize(fyne.NewSize(800, 600))
	w.SetFixedSize(true)
	w.CenterOnScreen()

	b := container.NewVBox()

	getTomlUPP()
	var temp []int
	for n := range gUPP {
		temp = append(temp, n)
	}
	sort.Ints(temp)

	for _, number := range temp {
		upp := gUPP[number]

		nameLabel := widget.NewLabel(fmt.Sprintf("%-4d %s", number, upp.Name))
		nameLabel.TextStyle.Monospace = true

		paramEntry[number] = widget.NewEntry()
		paramEntry[number].TextStyle.Monospace = true
		paramEntry[number].SetText(upp.Value)
		paramEntry[number].OnChanged = func(str string) {
			statusLabel.SetText(upp.Hint)
		}

		line := container.NewGridWithColumns(2, nameLabel, paramEntry[number])
		b.Add(line)
	}
	boxScrollUPP := container.NewVScroll(b)                                                             // + крутилку
	boxScrollLayoutUPP := container.New(layout.NewGridWrapLayout(fyne.NewSize(770, 550)), boxScrollUPP) // чтобы не расползались, нужно место для кнопок

	readButton := widget.NewButton("УПП БУ", nil)
	writeButton := widget.NewButton("Записать", func() {
		// проверить введенные данные
		for number := range gUPP {
			if !checkValueUPP(paramEntry[number].Text, gUPP[number].Hint) {
				statusLabel.SetText(fmt.Sprintf("Неверное значение параметра: «%s»", gUPP[number].Name))
				return
			}
		}
		// записать все в gUPP
		for number, val := range gUPP {
			fmt.Println(val.Value)
			val.Value = paramEntry[number].Text
			gUPP[number] = val
		}
		if err, number := writeUPPtoBU(); err != nil {
			statusLabel.SetText(fmt.Sprintf("Ошибка установки значения: «%s»", gUPP[number].Name))
		} else {
			writeTomlUPP() //переименовать todo
			statusLabel.SetText("УПП записаны успешно")
		}
	})

	boxButtons := container.NewHBox(readButton, layout.NewSpacer(), writeButton)
	boxBottom := container.NewVBox(statusLabel, boxButtons)

	boxButtonsLayout := container.New(layout.NewGridWrapLayout(fyne.NewSize(800, 80)), boxBottom) // чтобы не расползались кнопки при растягивании бокса

	box := container.NewVBox(boxScrollLayoutUPP, boxButtonsLayout)

	w.SetContent(box)
	w.Show() // ShowAndRun -- panic!
}
