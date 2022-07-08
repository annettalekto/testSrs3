package main

import (
	"fmt"
	"os/exec"
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
)

/* todo
- при вводе цифр в entry как определить конец ввода? Как узнать о нажатии клавиши?
- при вводе цифр все запятые менять на точки, сделать общую функцию
*/

var gVersion, gYear, gProgramName string

func main() {
	gVersion, gYear = "1.0.0", "2022 г." // todo править при изменениях
	gProgramName = "Электронная имитация параметров"

	// Инит
	// initIPK()
	// initDevice() ,??
	// запросить данные УПП!

	err := can25.Init(0x1F, 0x16)
	if err != nil {
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
	idSpeed1:        false,
	idSpeed2:        false,
	idAcceleration1: false,
	idAcceleration2: false,
	idPressure:      false,
	idDistance:      false,
	idTimeBU:        false,
	idALS:           false,
	idBin:           false,
	idCodeIF:        false,
}

func getMsgCAN() {
	waitTime := time.Second * 1
	// var msg candev.Message

	msg, err := can25.GetMsgByIDList(idListCAN, waitTime)
	if err == nil {
		mapDataCAN[msg.ID] = msg.Data
		fmt.Println("Что то полезное")
	}
}

func dataCANToString(id uint32, data [8]byte) (str string) {

	switch id {
	case idSpeed1, idSpeed2:
		f := byteToSpeed(data)
		if id == idSpeed1 {
			str = fmt.Sprintf("Скорость 1 канал (км\\ч): %.0f", f)
		} else {
			str = fmt.Sprintf("Скорость 2 канал (км\\ч): %.0f", f)
		}

	case idAcceleration1, idAcceleration2:
		f := byteToAcceleration(data)
		if id == idAcceleration1 {
			str = fmt.Sprintf("Ускорение 1 канал (м\\с²): %.0f", f)
		} else {
			str = fmt.Sprintf("Ускорение 2 канал (м\\с²): %.0f", f)
		}

	case 1, 2, 3:
		tm, tc, gr := byteToPressure(data)

		if id == 1 {
			str = fmt.Sprintf("Давление в ТМ (кг/см²): %.0f", tm)
		} else if id == 2 {
			str = fmt.Sprintf("Давление в ТЦ (кг/см²): %.0f", tc)
		} else if id == 3 {
			str = fmt.Sprintf("Давление в ГР (кг/см²): %.0f", gr)
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

	labelSpeed1 := widget.NewLabel("")
	labelSpeed2 := widget.NewLabel("")
	labelAcceleration1 := widget.NewLabel("")
	labelAcceleration2 := widget.NewLabel("")
	labelPress1 := widget.NewLabel("")
	labelPress2 := widget.NewLabel("")
	labelPress3 := widget.NewLabel("")
	labelDistance := widget.NewLabel("")
	labelTimeBU := widget.NewLabel("")

	// обновление данных
	labelTimeBU.SetText(dataCANToString(idTimeBU, mapDataCAN[idTimeBU]))
	labelSpeed1.SetText(dataCANToString(idSpeed1, mapDataCAN[idSpeed1]))
	labelSpeed2.SetText(dataCANToString(idSpeed2, mapDataCAN[idSpeed2]))
	labelAcceleration1.SetText(dataCANToString(idAcceleration1, mapDataCAN[idAcceleration1]))
	labelAcceleration2.SetText(dataCANToString(idAcceleration2, mapDataCAN[idAcceleration2]))
	labelPress1.SetText(dataCANToString(1, mapDataCAN[1]))
	labelPress2.SetText(dataCANToString(2, mapDataCAN[2]))
	labelPress3.SetText(dataCANToString(3, mapDataCAN[3]))
	labelDistance.SetText(dataCANToString(idDistance, mapDataCAN[idDistance]))

	// получение данных
	go func() {
		for {
			getMsgCAN()
			time.Sleep(100 * time.Millisecond)
		}
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

	// Совместное-раздельное управление
	separately := binding.NewBool() // false вместе
	direction1 := uint8(1 /*ipk.MotionOnward*/)
	direction2 := uint8(1 /*ipk.MotionOnward*/)
	speed1, speed2, accel1, accel2 := float64(0), float64(0), float64(0), float64(0)

	// debug: todo
	// sp.SetMotion(direction1) // todo править библиотеку!
	// sp.SetSpeed(speed1, speed2)
	// sp.SetAcceleration(accel1, accel2)

	fmt.Println(direction1, direction2, speed1, speed2, accel1, accel2) // todo

	// ------------------------- box 1 ----------------------------

	var style fyne.TextStyle
	style.Bold = true
	textSpeed := widget.NewLabelWithStyle("Частотные каналы:", fyne.TextAlignCenter, style)

	dummy := widget.NewLabel("")
	entrySpeed1 := newNumericalEntry() // todo заменять запятую на точку? игнорировать запятую?
	entrySpeed2 := newNumericalEntry()
	entryAccel1 := newNumericalEntry()
	entryAccel2 := newNumericalEntry()
	separatlyCheck := widget.NewCheckWithData("Раздельное управление", separately)

	// обработка скорости
	entrySpeed1.Entry.TextStyle.Monospace = true
	entrySpeed1.Entry.SetPlaceHolder("0.00")
	entrySpeed1.Entry.OnChanged = func(str string) {
		speed1, _ = strconv.ParseFloat(str, 64) // todo err
		sep, _ := separately.Get()              // !sep если управление не раздельное
		if !sep {
			entrySpeed2.Entry.SetText(str) // тоже вводим в поле второго канала скорости
		}
	}

	entrySpeed2.Entry.TextStyle.Monospace = true
	entrySpeed2.Entry.SetPlaceHolder("0.00")
	entrySpeed2.Entry.OnChanged = func(str string) {
		speed2, _ = strconv.ParseFloat(str, 64)
		sep, _ := separately.Get()
		if !sep {
			entrySpeed1.Entry.SetText(str)
		}
	}

	if entrySpeed1.Entered || entrySpeed2.Entered {
		// sp.SetSpeed(speed1, speed2)
	}

	// обработка ускорения
	entryAccel1.Entry.TextStyle.Monospace = true
	entryAccel1.Entry.SetPlaceHolder("0.00")
	entryAccel1.Entry.OnChanged = func(str string) {
		accel1, _ = strconv.ParseFloat(str, 64)
		sep, _ := separately.Get()
		if !sep {
			entryAccel2.Entry.SetText(str)
		}
	}

	entryAccel2.Entry.TextStyle.Monospace = true
	entryAccel2.Entry.SetPlaceHolder("0.00")
	entryAccel2.Entry.OnChanged = func(str string) {
		accel2, _ = strconv.ParseFloat(str, 64)
		sep, _ := separately.Get()
		if !sep {
			entryAccel1.Entry.SetText(str)
		}
	}

	if entryAccel1.Entered || entryAccel2.Entered {
		// sp.SetAcceleration(accel1, accel2)
	}

	// обработка направления
	directionChoice := []string{"Вперед", "Назад"}
	var selectDirection1, selectDirection2 *widget.Select

	selectDirection1 = widget.NewSelect(directionChoice, func(s string) {
		sep, _ := separately.Get()
		if s == "Вперед" {
			direction1 = 1                                     /*ipk.MotionOnward*/
			if !sep && selectDirection2.SelectedIndex() != 0 { // бесконечный вызов!
				selectDirection2.SetSelectedIndex(0)
			}
		} else {
			direction1 = 2 /*ipk.MotionBackwards*/
			if !sep && selectDirection1.SelectedIndex() != 1 {
				selectDirection2.SetSelectedIndex(1)
			}
		}
		// sp.SetMotion(direction1)
	})
	selectDirection2 = widget.NewSelect(directionChoice, func(s string) {
		sep, _ := separately.Get()
		if s == "Вперед" {
			direction2 = 1
			if !sep && selectDirection2.SelectedIndex() != 0 {
				selectDirection2.SetSelectedIndex(0)
			}
		} else {
			direction2 = 2 /*ipk.MotionBackwards*/
			if !sep && selectDirection1.SelectedIndex() != 1 {
				selectDirection1.SetSelectedIndex(1)
			}
		}
		// sp.SetMotion(direction1)
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

	boxSpeed := container.NewVBox(textSpeed, box1, separatlyCheck)

	// ------------------------- box 2 ----------------------------
	// Путь:

	textMileage := widget.NewLabelWithStyle("Имитация пути:", fyne.TextAlignCenter, style)

	entryMileage := newNumericalEntry()
	entryMileage.Entry.TextStyle.Monospace = true
	entryMileage.Entry.SetPlaceHolder("20.000")
	buttonMileage := widget.NewButton("Пуск", func() {
		// todo запуск
	})
	labelMileage := widget.NewLabel("10.000") // todo обновлять если запущена проверка
	// byteDistance(mapDataCAN[idDistance])

	box3 := container.NewGridWithColumns(
		3,
		widget.NewLabel("Дистанция (км):"), entryMileage, buttonMileage,
		widget.NewLabel("Текущая (км):"), labelMileage,
	)
	boxMileage := container.NewVBox(textMileage, box3)

	// ------------------------- box 3 ----------------------------
	// Давление

	textPress := widget.NewLabelWithStyle("Аналоговые каналы:", fyne.TextAlignCenter, style)

	entryPress1 := newNumericalEntry()
	entryPress1.Entry.TextStyle.Monospace = true
	entryPress1.Entry.SetPlaceHolder("0.00") // todo ограничить 10 атм - добавить метод проверяющий max

	entryPress2 := newNumericalEntry()
	entryPress2.Entry.TextStyle.Monospace = true
	entryPress2.Entry.SetPlaceHolder("0.00") // 20 атм

	entryPress3 := newNumericalEntry()
	entryPress3.Entry.TextStyle.Monospace = true
	entryPress3.Entry.SetPlaceHolder("0.00") // 20 атм

	box4 := container.NewGridWithColumns(
		2,
		widget.NewLabel("Канал 1 (кгс/см²):"), entryPress1,
		widget.NewLabel("Канал 2 (кгс/см²):"), entryPress2,
		widget.NewLabel("Канал 3 (кгс/см²):"), entryPress3,
	)
	boxPress := container.NewVBox(textPress, box4, dummy)

	boxAll := container.NewVBox(boxSpeed, boxMileage, boxPress)
	// boxSpeedAndMileage := container.NewVSplit(boxSpeed, boxMileage)
	// boxAll := container.NewVSplit(boxSpeedAndMileage, boxPress)

	box := container.NewHBox(boxAll, dummy)

	return box //container.New(layout.NewGridWrapLayout(fyne.NewSize(450, 850)), box)
}

// коды РЦ (Сигналы ИФ) +
// Вых.БУ: 50В, 10В
func outputSignals() fyne.CanvasObject {
	// dummy := widget.NewLabel("")

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
		fmt.Println(s)
	})
	// radio.Horizontal = true
	boxCode := container.NewVBox(labelCode, radio)

	labelOut := widget.NewLabelWithStyle("Вых.БУ:", fyne.TextAlignCenter, style)
	// 50V
	checkG := widget.NewCheck("З", nil)            // З		0
	checkY := widget.NewCheck("Ж", nil)            // Ж		1
	checkRY := widget.NewCheck("КЖ", nil)          // КЖ	2
	checkR := widget.NewCheck("К", nil)            // К		3
	checkW := widget.NewCheck("Б", nil)            // Б		4
	checkEPK1 := widget.NewCheck("ЭПК1", nil)      // ЭПК1	5
	checkIF := widget.NewCheck("ИФ", nil)          // ИФ	6
	checkTracktion := widget.NewCheck("Тяга", nil) // Тяга	7
	boxOut50V := container.NewVBox(checkG, checkY, checkRY, checkR, checkW, checkEPK1, checkIF, checkTracktion)
	// 10V
	checkLP := widget.NewCheck("ЛП", nil)              // 1
	checkButtonUhod := widget.NewCheck("кн.Уход", nil) // 3
	checkEPK := widget.NewCheck("ЭПК", nil)            // 5
	checkPowerBU := widget.NewCheck("Пит.БУ", nil)     // 7
	checkKeyEPK := widget.NewCheck("Ключ ЭПК", nil)    // 9
	boxOut10V := container.NewVBox(checkLP, checkButtonUhod, checkEPK, checkPowerBU, checkKeyEPK)

	boxOut := container.NewVBox(labelOut, boxOut10V, boxOut50V)
	box := container.NewHBox(boxOut, boxCode)

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

	// Доп. параметры:
	numberTeeth, _ := strconv.ParseInt(valueNumberTeeth, 10, 32)
	diameter, _ := strconv.ParseInt(valueBandageDiameter1, 10, 32)
	// sp.Init(fcs, uint32(numberTeeth), uint32(diameter))

	entryDiameter := newNumericalEntry()
	diameter = 42 // todo ОТЛАДКА
	numberTeeth = 1350
	entryDiameter.Entry.SetPlaceHolder(fmt.Sprintf("%d", diameter))
	entryNumberTeeth := newNumericalEntry()
	entryNumberTeeth.Entry.SetPlaceHolder(fmt.Sprintf("%d", numberTeeth))

	box := container.NewHBox(checkPower, checkTurt, widget.NewLabel("Кол-во зубьев: "), entryNumberTeeth, widget.NewLabel("Диаметр: "), entryDiameter)
	return container.New(layout.NewGridWrapLayout(fyne.NewSize(400, 35)), box)
}

//---------------------------------------------------------------------------
