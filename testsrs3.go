package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
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
)

/* todo
+ при вводе цифр в entry как определить конец ввода? Как узнать о нажатии клавиши?
- при вводе цифр все запятые менять на точки, сделать общую функцию
*/

var gVersion, gYear, gProgramName string

func main() {
	gVersion, gYear = "1.0.0", "2022 г." // todo править при изменениях
	gProgramName = "Электронная имитация параметров"

	// Инит
	debugGetUPP()
	initIPK()
	// initDevice()
	// запросить данные УПП!

	err := can25.Init(0x1F, 0x16)
	if err != nil {
		fmt.Printf("Ошибка инициализации CAN: %v\n", err)
		// return // todo запускать форму при отсутствие can?
	}
	can25.Run()
	defer can25.Stop()

	// Форма
	a := app.New()
	w := a.NewWindow(gProgramName)
	// w.Resize(fyne.NewSize(800, 600))
	w.CenterOnScreen()
	w.SetMaster()
	// dummy := widget.NewLabel("  ")

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

	// Элементы
	boxSpeed := speed()
	boxOutput := outputSignals()
	box1 := container.NewHSplit(boxSpeed, boxOutput)

	boxInput := inputSignals()
	box2 := container.NewVSplit(box1, boxInput)

	top := top()
	box3 := container.NewVSplit(top, box2)

	boxCAN := dataCAN()
	box := container.NewHSplit(box3, boxCAN)

	w.SetContent(box)
	w.ShowAndRun()
}

var currentTheme bool // светлая тема false

func changeTheme(a fyne.App) {
	currentTheme = !currentTheme

	if currentTheme {
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

//---------------------------------------------------------------------------
// Данные CAN

const (
	idSpeed1        = 0x5E5
	idSpeed2        = 0x5E6
	idAcceleration1 = 0x5E3
	idAcceleration2 = 0x5E4
	idPressure      = 0x5FC
	idDistance      = 0x5C6
	idTimeBU        = 0xC7
	idALS           = 0x50
	idBin           = 0x5F8
	idCodeIF        = 0x5C5
)

var mapDataCAN map[uint32][8]byte
var idListCAN = map[uint32]bool{
	idSpeed1: true,
	idSpeed2: true,
	/*idAcceleration1: false,
	idAcceleration2: false,
	idPressure:      true,
	idDistance:      false,
	idTimeBU:        false,
	idALS:           false,
	idBin:           false,
	idCodeIF:        false,*/
}

func getMsgCAN() {
	waitTime := time.Millisecond * 100

	msg, err := can25.GetMsgByIDList(idListCAN, waitTime)
	if err == nil {
		mapDataCAN[msg.ID] = msg.Data
	}
}

func dataCANToString(id uint32, data [8]byte) (str string) {

	switch id {
	case idSpeed1, idSpeed2:
		f := byteToSpeed(data)
		if id == idSpeed1 {
			str = fmt.Sprintf("Скорость 1 (км/ч): %.0f", f)
		} else {
			str = fmt.Sprintf("Скорость 2 (км/ч): %.0f", f)
		}

	case idAcceleration1, idAcceleration2:
		f := byteToAcceleration(data)
		if id == idAcceleration1 {
			str = fmt.Sprintf("Ускорение 1 (м/с²): %.0f", f)
		} else {
			str = fmt.Sprintf("Ускорение 2 (м/с²): %.0f", f)
		}

	case 1, 2, 3:
		tm, tc, gr := byteToPressure(data)

		if id == 1 {
			str = fmt.Sprintf("Давление ТМ (кг/см²): %.0f", tm)
		} else if id == 2 {
			str = fmt.Sprintf("Давление ТЦ (кг/см²): %.0f", tc)
		} else if id == 3 {
			str = fmt.Sprintf("Давление ГР (кг/см²): %.0f", gr)
		}

	case idDistance:
		u := byteDistance(data)
		str = fmt.Sprintf("Дистанция (м): %d", u)

	case idTimeBU:
		t := byteToTimeBU(data)
		str = fmt.Sprintf("Время БУ: %s", t.Format("02.01.2006 15:04"))

	default:
	}
	return
}

func dataCAN() fyne.CanvasObject {
	mapDataCAN = make(map[uint32][8]byte) // скопище байтов из CAN

	var style fyne.TextStyle
	style.Bold = true
	text := widget.NewLabelWithStyle("Данные CAN:", fyne.TextAlignCenter, style)

	labelTimeBU := widget.NewLabel("")
	labelSpeed1 := widget.NewLabel("")
	labelSpeed2 := widget.NewLabel("")
	labelAcceleration1 := widget.NewLabel("")
	labelAcceleration2 := widget.NewLabel("")
	labelPress1 := widget.NewLabel("")
	labelPress2 := widget.NewLabel("")
	labelPress3 := widget.NewLabel("")
	labelDistance := widget.NewLabel("")

	timeCheckDone := make(chan int) // признак того что результат готов

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
					mapDataCAN[msg.ID] = msg.Data
					// defer can25.CloseMsgChannelCopy(idx)

				}
			default:
				runtime.Gosched()
			}

			// обновление данных
			labelTimeBU.SetText(dataCANToString(idTimeBU, mapDataCAN[idTimeBU])) // желательно прочерк если нет в CAN
			labelSpeed1.SetText(dataCANToString(idSpeed1, mapDataCAN[idSpeed1]))
			labelSpeed2.SetText(dataCANToString(idSpeed2, mapDataCAN[idSpeed2]))
			labelAcceleration1.SetText(dataCANToString(idAcceleration1, mapDataCAN[idAcceleration1]))
			labelAcceleration2.SetText(dataCANToString(idAcceleration2, mapDataCAN[idAcceleration2]))
			labelPress1.SetText(dataCANToString(1, mapDataCAN[idPressure]))
			labelPress2.SetText(dataCANToString(2, mapDataCAN[idPressure]))
			labelPress3.SetText(dataCANToString(3, mapDataCAN[idPressure]))
			labelDistance.SetText(dataCANToString(idDistance, mapDataCAN[idDistance]))
			// time.Sleep(200 * time.Millisecond)
		}
		timeCheckDone <- 1
	}()

	box := container.NewVBox(text,
		labelSpeed1,
		labelSpeed2,
		labelAcceleration1,
		labelAcceleration2,
		labelPress1,
		labelPress2,
		labelPress3,
		labelDistance,
		labelTimeBU,
	)

	// box := container.New(layout.NewGridWrapLayout(fyne.NewSize(300, 800)), box1)
	return container.NewVScroll(box)
}

//---------------------------------------------------------------------------
// ИНТЕРФЕЙС

// Скорость, дистанция, давление
func speed() fyne.CanvasObject {
	var err error

	// ------------------------- box 1 ----------------------------

	separately := binding.NewBool() // cовместное-раздельное управление: false вместе
	direction1 := uint8(ipk.MotionOnward)
	direction2 := uint8(ipk.MotionOnward)
	speed1, speed2, accel1, accel2 := float64(0), float64(0), float64(0), float64(0)

	var style fyne.TextStyle
	style.Bold = true
	textSpeed := widget.NewLabelWithStyle("Имитация движения:", fyne.TextAlignCenter, style)

	dummy := widget.NewLabel("")
	entrySpeed1 := newNumericalEntry() // todo заменять запятую на точку? игнорировать запятую?
	entrySpeed2 := newNumericalEntry()
	entryAccel1 := newNumericalEntry()
	entryAccel2 := newNumericalEntry()
	separatlyCheck := widget.NewCheckWithData("Раздельное управление", separately)

	// ---------------------- обработка скорости
	entrySpeed1.Entry.TextStyle.Monospace = true
	entrySpeed1.Entry.SetPlaceHolder("0.00")
	entrySpeed1.Entry.OnChanged = func(str string) {
		speed1, err = strconv.ParseFloat(str, 64)
		if err != nil {
			fmt.Printf("Ошибка перевода строки в число (скорость 1)\n")
		}
		if sep, _ := separately.Get(); !sep { // !не раздельное управление
			speed2 = speed1                // тоже в переменную
			entrySpeed2.Entry.SetText(str) // и в поле второго канала скорости
		}
	}

	entrySpeed2.Entry.TextStyle.Monospace = true
	entrySpeed2.Entry.SetPlaceHolder("0.00")
	entrySpeed2.Entry.OnChanged = func(str string) {
		speed2, err = strconv.ParseFloat(str, 64)
		if err != nil {
			fmt.Printf("Ошибка перевода строки в число (скорость 2)\n")
		}
		if sep, _ := separately.Get(); !sep {
			speed1 = speed2
			entrySpeed1.Entry.SetText(str)
		}

	}

	// ---------------------- обработка ускорения
	entryAccel1.Entry.TextStyle.Monospace = true
	entryAccel1.Entry.SetPlaceHolder("0.00")
	entryAccel1.Entry.OnChanged = func(str string) {
		accel1, err = strconv.ParseFloat(str, 64)
		if err != nil {
			fmt.Printf("Ошибка перевода строки в число (ускорение 1)\n")
		}
		if sep, _ := separately.Get(); !sep {
			accel2 = accel1
			entryAccel2.Entry.SetText(str)
		}
	}

	entryAccel2.Entry.TextStyle.Monospace = true
	entryAccel2.Entry.SetPlaceHolder("0.00")
	entryAccel2.Entry.OnChanged = func(str string) {
		accel2, err = strconv.ParseFloat(str, 64)
		if err != nil {
			fmt.Printf("Ошибка перевода строки в число (ускорение 2)\n")
		}
		if sep, _ := separately.Get(); !sep {
			accel1 = accel2
			entryAccel1.Entry.SetText(str)
		}
	}

	// ---------------------- обработка направления
	directionChoice := []string{"Вперед", "Назад"}
	var selectDirection1, selectDirection2 *widget.Select

	selectDirection1 = widget.NewSelect(directionChoice, func(s string) {
		sep, _ := separately.Get()
		if s == "Вперед" {
			direction1 = ipk.MotionOnward
			if !sep && selectDirection2.SelectedIndex() != 0 { // бесконечный вызов!
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
		if s == "Вперед" {
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

	selectDirection1.SetSelectedIndex(0) //"Вперед")
	selectDirection2.SetSelectedIndex(0) //"Вперед")

	box1 := container.NewGridWithColumns(
		3,
		dummy, widget.NewLabel("Канал 1"), widget.NewLabel("Канал 2"),
		widget.NewLabel("Скорость (км/ч):"), entrySpeed1, entrySpeed2,
		widget.NewLabel("Ускорение (м/с²):"), entryAccel1, entryAccel2,
		widget.NewLabel("Направление:"), selectDirection1, selectDirection2,
	)

	// ---------------------- Доп. параметры
	numberTeeth, _ := strconv.Atoi(valueNumberTeeth)    // это значения УПП,
	diameter, _ := strconv.Atoi(valueBandageDiameter1)  // установленные на блоке
	sp.Init(fcs, uint32(numberTeeth), uint32(diameter)) // их используем как предустановку

	entryDiameter := newNumericalEntry()
	entryDiameter.Entry.TextStyle.Monospace = true
	entryDiameter.Entry.SetText(fmt.Sprintf("%d", diameter))
	entryDiameter.Entry.OnChanged = func(str string) {
		if val, err := strconv.Atoi(str); err != nil {
			fmt.Printf("Ошибка перевода строки в число (диаметр бандажа)\n")
		} else {
			diameter = val
			sp.Init(fcs, uint32(numberTeeth), uint32(diameter)) // используем введенное значение
		}
	}

	entryNumberTeeth := newNumericalEntry()
	entryNumberTeeth.Entry.TextStyle.Monospace = true
	entryNumberTeeth.Entry.SetText(fmt.Sprintf("%d", numberTeeth))
	entryNumberTeeth.Entry.OnChanged = func(str string) {
		if val, err := strconv.Atoi(str); err != nil {
			fmt.Printf("Ошибка перевода строки в число (количество зубьев)\n")
		} else {
			numberTeeth = val
			sp.Init(fcs, uint32(numberTeeth), uint32(diameter))
		}
	}
	addParam := container.NewHBox(widget.NewLabel("Кол-во зубьев:     "), entryNumberTeeth, widget.NewLabel("Диаметр (мм):  "), entryDiameter)

	boxSpeed := container.NewVBox(textSpeed, box1, separatlyCheck)

	// ------------------------- box 2 ----------------------------
	// Путь:

	startDistanceCheck := false
	distance := 0
	startDistance := uint32(0)

	textMileage := widget.NewLabelWithStyle("Имитация пути (м):", fyne.TextAlignCenter, style)

	entryMileage := newNumericalEntry()
	entryMileage.Entry.TextStyle.Monospace = true
	entryMileage.Entry.SetPlaceHolder("20000")
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

	box3 := container.NewGridWithColumns(
		3,
		widget.NewLabel("Дистанция:"), entryMileage, buttonMileage,
		widget.NewLabel("Текущая:"), labelMileage,
	)
	boxMileage := container.NewVBox(textMileage, box3)

	// ------------------------- box 3 ----------------------------
	// Давление

	var press1, press2, press3 float64

	textPress := widget.NewLabelWithStyle("Имитация давления (кгс/см²):", fyne.TextAlignCenter, style)

	entryPress1 := newNumericalEntry()
	entryPress1.Entry.TextStyle.Monospace = true
	entryPress1.Entry.SetPlaceHolder("0.00") // todo ограничить 10 атм - добавить метод проверяющий max
	entryPress1.Entry.OnChanged = func(str string) {
		press1, err = strconv.ParseFloat(str, 64)
		if err != nil {
			fmt.Printf("Ошибка перевода строки в число (давление 1)\n")
		}
	}

	entryPress2 := newNumericalEntry()
	entryPress2.Entry.TextStyle.Monospace = true
	entryPress2.Entry.SetPlaceHolder("0.00") // из УПП ~ 16 атм
	entryPress2.Entry.OnChanged = func(str string) {
		press2, err = strconv.ParseFloat(str, 64)
		if err != nil {
			fmt.Printf("Ошибка перевода строки в число (давление 2)\n")
		}
	}

	entryPress3 := newNumericalEntry()
	entryPress3.Entry.TextStyle.Monospace = true
	entryPress3.Entry.SetPlaceHolder("0.00") // 20 атм
	entryPress3.Entry.OnChanged = func(str string) {
		press3, err = strconv.ParseFloat(str, 64)
		if err != nil {
			fmt.Printf("Ошибка перевода строки в число (давление 3)\n")
		}
	}

	box4 := container.NewGridWithColumns(
		3,
		widget.NewLabel("Канал 1:"), widget.NewLabel("Канал 2:"), widget.NewLabel("Канал 3:"),
		entryPress1, entryPress2, entryPress3,
	)
	boxPress := container.NewVBox(textPress, box4)

	boxAll := container.NewVBox(boxSpeed, boxMileage, widget.NewLabel("Параметры имитатора:"), addParam, boxPress, dummy)
	// boxSpeedAndMileage := container.NewVSplit(boxSpeed, boxMileage)
	// boxAll := container.NewVSplit(boxSpeedAndMileage, boxPress)

	box := container.NewHBox(dummy, boxAll, dummy)

	// -------------------- установка значений -----------------------

	// если Enter был нажат, значит ввод закончен
	go func() {
		for {
			if entrySpeed1.Entered || entrySpeed2.Entered {
				err = sp.SetSpeed(speed1, speed2)
				if err != nil {
					fmt.Printf("Ошибка установки скорости")
				}
				entrySpeed1.Entered = false
				entrySpeed2.Entered = false
				fmt.Printf("Скорость: %.1f %.1f км/ч (%v)\n", speed1, speed2, err)
			}

			if entryAccel1.Entered || entryAccel2.Entered {
				err = sp.SetAcceleration(accel1, accel2)
				if err != nil {
					fmt.Printf("Ошибка установки ускорения\n")
				}
				entryAccel1.Entered = false
				entryAccel2.Entered = false
				fmt.Printf("Ускорение: %.1f %.1f м/с2 (%v)\n", accel1, accel2, err)
			}

			if startDistanceCheck {
				m, _, err := sp.GetWay()
				if err != nil {
					fmt.Printf("Не получено значение пути с ИПК\n")
				}
				labelMileage.SetText(fmt.Sprintf("%d", startDistance-m))
			}

			if entryPress1.Entered {
				err = channel1.Set(press1)
				if err != nil {
					fmt.Printf("Ошибка установки давления 1\n")
				}
				fmt.Printf("Давление 1: %.1f кгс/см2 (%v)\n", press1, err)
				entryPress1.Entered = false
			}
			if entryPress2.Entered {
				err = channel2.Set(press2)
				if err != nil {
					fmt.Printf("Ошибка установки давления 2\n")
				}
				fmt.Printf("Давление 2: %.1f кгс/см2 (%v)\n", press2, err)
				entryPress2.Entered = false
			}
			if entryPress3.Entered {
				err = channel3.Set(press3)
				if err != nil {
					fmt.Printf("Ошибка установки давления 3\n")
				}
				fmt.Printf("Давление 3: %.1f кгс/см2 (%v)\n", press3, err)
				entryPress3.Entered = false
			}

			time.Sleep(time.Second)
		}
	}()

	return box //container.New(layout.NewGridWrapLayout(fyne.NewSize(450, 850)), box)
}

// коды РЦ (Сигналы ИФ) +
// Вых.БУ: 50В, 10В
func outputSignals() fyne.CanvasObject {
	// dummy := widget.NewLabel("")
	var err error
	var style fyne.TextStyle
	style.Bold = true
	labelCode := widget.NewLabelWithStyle("Коды РЦ:", fyne.TextAlignCenter, style)

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
			err = fds.SetIF(ipk.IFDisable) // ?
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
	boxCode := container.NewVBox(labelCode, radio)

	labelOut := widget.NewLabelWithStyle("Вых.БУ:", fyne.TextAlignCenter, style)
	// 50V
	checkG := widget.NewCheck("З", func(on bool) {
		if on {
			err = fds.Set50V(0, true)
		} else {
			err = fds.Set50V(0, false)
		}
		fmt.Printf("Двоичные выходы 50В: 0=%v З (%v)\n", on, err)
	})
	checkY := widget.NewCheck("Ж", func(on bool) {
		if on {
			err = fds.Set50V(1, true)
		} else {
			err = fds.Set50V(1, false)
		}
		fmt.Printf("Двоичные выходы 50В: 1=%v Ж(%v)\n", on, err)
	})
	checkRY := widget.NewCheck("КЖ", func(on bool) {
		if on {
			err = fds.Set50V(2, true)
		} else {
			err = fds.Set50V(2, false)
		}
		fmt.Printf("Двоичные выходы 50В: 2=%v КЖ (%v)\n", on, err)
	})
	checkR := widget.NewCheck("К", func(on bool) {
		if on {
			err = fds.Set50V(3, true)
		} else {
			err = fds.Set50V(3, false)
		}
		fmt.Printf("Двоичные выходы 50В: 3=%v К (%v)\n", on, err)
	})
	checkW := widget.NewCheck("Б", func(on bool) {
		if on {
			err = fds.Set50V(4, true)
		} else {
			err = fds.Set50V(4, false)
		}
		fmt.Printf("Двоичные выходы 50В: 4=%v Б (%v)\n", on, err)
	})
	checkEPK1 := widget.NewCheck("ЭПК1", func(on bool) {
		if on {
			err = fds.Set50V(5, true)
		} else {
			err = fds.Set50V(5, false)
		}
		fmt.Printf("Двоичные выходы 50В: 5=%v ЭПК1 (%v)\n", on, err)
	})
	checkIF := widget.NewCheck("ИФ", func(on bool) {
		if on {
			err = fds.Set50V(6, true)
		} else {
			err = fds.Set50V(6, false)
		}
		fmt.Printf("Двоичные выходы 50В: 6=%v ИФ (%v)\n", on, err)
	})
	checkTracktion := widget.NewCheck("Тяга", func(on bool) {
		if on {
			err = fds.Set50V(7, true)
		} else {
			err = fds.Set50V(7, false)
		}
		fmt.Printf("Двоичные выходы 50В: 7=%v Тяга (%v)\n", on, err)
	})
	boxOut50V := container.NewVBox(checkG, checkY, checkRY, checkR, checkW, checkEPK1, checkIF, checkTracktion)
	// 10V
	checkLP := widget.NewCheck("ЛП", func(on bool) {
		if on {
			err = fds.Set10V(1, true)
		} else {
			err = fds.Set10V(1, false)
		}
		fmt.Printf("Двоичные выходы 10В: 1=%v ЛП (%v)\n", on, err)
	})
	checkButtonUhod := widget.NewCheck("кн.Уход", func(on bool) {
		if on {
			err = fds.Set10V(3, true)
		} else {
			err = fds.Set10V(3, false)
		}
		fmt.Printf("Двоичные выходы 10В: 3=%v кн.Уход (%v)\n", on, err)
	})
	checkEPK := widget.NewCheck("ЭПК", func(on bool) {
		if on {
			err = fds.Set10V(5, true)
		} else {
			err = fds.Set10V(5, false)
		}
		fmt.Printf("Двоичные выходы 10В: 5=%v ЭПК (%v)\n", on, err)
	})
	checkPowerBU := widget.NewCheck("Пит.БУ", func(on bool) {
		if on {
			err = fds.Set10V(7, true)
		} else {
			err = fds.Set10V(7, false)
		}
		fmt.Printf("Двоичные выходы 10В: 7=%v Пит.БУ (%v)\n", on, err)
	})
	checkKeyEPK := widget.NewCheck("Ключ ЭПК", func(on bool) {
		if on {
			err = fds.Set10V(9, true)
		} else {
			err = fds.Set10V(9, false)
		}
		fmt.Printf("Двоичные выходы 10В: 9=%v Ключ ЭПК (%v)\n", on, err)
	})
	boxOut10V := container.NewVBox(checkLP, checkButtonUhod, checkEPK, checkPowerBU, checkKeyEPK)

	boxOut := container.NewVBox(labelOut, boxOut10V, boxOut50V)
	box := container.NewHSplit(boxOut, boxCode)

	return box
}

// Уставки, входы БУС = считать
func inputSignals() fyne.CanvasObject {
	// dummy := widget.NewLabel("")

	var style fyne.TextStyle
	style.Bold = true
	labelRelay := widget.NewLabelWithStyle("Реле превышения уставок:", fyne.TextAlignLeading, style)

	check1 := widget.NewCheck("1", nil)
	check20 := widget.NewCheck("20", nil)
	check80 := widget.NewCheck("80", nil)
	check60 := widget.NewCheck("60", nil)
	check30 := widget.NewCheck("30", nil)
	boxRelay := container.NewHBox(check1, check20, check80, check60, check30)

	// labelBUS := widget.NewLabel("Входы БУС:")
	checkPSS2 := widget.NewCheck("ПСС2", nil)
	checkUhod2 := widget.NewCheck("Уход 2", nil)
	checkPowerEPK := widget.NewCheck("Пит.ЭПК", nil)
	checkPB2 := widget.NewCheck("РБ2", nil)
	checkEVM := widget.NewCheck("ЭВМ", nil)
	boxBUS := container.NewHBox(checkPSS2, checkUhod2, checkPowerEPK, checkPB2, checkEVM)

	box := container.NewHBox(boxRelay, boxBUS)

	return container.NewVBox(labelRelay, box)
}

func top() fyne.CanvasObject {

	powerKPD := binding.NewBool() // питание включается при старте? todo
	powerKPD.Set(true)            // устанавливается в начале
	checkPower := widget.NewCheckWithData("Питание КПД", powerKPD)

	turn := binding.NewBool()
	turn.Set(false)
	checkTurt := widget.NewCheckWithData("Режим обслуживания", turn)

	box := container.NewHBox(checkPower, checkTurt)
	return container.New(layout.NewGridWrapLayout(fyne.NewSize(400, 35)), box)
}

//---------------------------------------------------------------------------
