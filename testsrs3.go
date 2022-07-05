package main

import (
	"fmt"
	"image/color"
	"ipk"
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
	initIPK()
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
	top := top()
	boxSpeed := speed()
	boxInput := inputSignals()
	boxOutput := outputSignals()
	boxSignalsIO := container.NewHSplit(boxOutput, boxInput)
	boxCAN := dataCAN()

	box1 := container.NewHSplit(boxSpeed, boxSignalsIO)
	box2 := container.NewVSplit(top, box1)
	box := container.NewHSplit(box2, boxCAN)

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

	text := canvas.NewText("	Данные CAN: ", color.Black)
	text.TextSize = 16

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
	labelSpeed1.SetText(dataCANToString(idSpeed1, mapDataCAN[idSpeed1]))
	labelSpeed2.SetText(dataCANToString(idSpeed2, mapDataCAN[idSpeed2]))
	labelAcceleration1.SetText(dataCANToString(idAcceleration1, mapDataCAN[idAcceleration1]))
	labelAcceleration2.SetText(dataCANToString(idAcceleration2, mapDataCAN[idAcceleration2]))
	labelPress1.SetText(dataCANToString(1, mapDataCAN[1]))
	labelPress2.SetText(dataCANToString(2, mapDataCAN[2]))
	labelPress3.SetText(dataCANToString(3, mapDataCAN[3]))
	labelDistance.SetText(dataCANToString(idDistance, mapDataCAN[idDistance]))
	labelTimeBU.SetText(dataCANToString(idTimeBU, mapDataCAN[idTimeBU]))

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
	separately := false // вместе
	direction1 := uint8(ipk.MotionOnward)
	direction2 := uint8(ipk.MotionOnward)
	speed1, speed2, accel1, accel2 := float64(0), float64(0), float64(0), float64(0)
	numberTeeth, _ := strconv.ParseInt(valueNumberTeeth, 10, 32)
	diameter, _ := strconv.ParseInt(valueBandageDiameter1, 10, 32)

	// debug: todo
	sp.SetMotion(direction1) // todo править библиотеку!
	sp.SetSpeed(speed1, speed2)
	sp.SetAcceleration(accel1, accel2)
	sp.Init(fcs, uint32(numberTeeth), uint32(diameter))

	fmt.Println(direction1, direction2, speed1, speed2, accel1, accel2) // todo

	selectbox := widget.NewSelect([]string{"Совместное", "Раздельное"}, func(s string) {
		if s == "Отдельное" {
			separately = true
			fmt.Println(separately)
		}
	})
	selectbox.SetSelected("Совместное")
	dummy := widget.NewLabel("")

	// Скорость (+ускорение и направление)
	textSpeed := canvas.NewText("	Частотные каналы: ", color.Black)
	textSpeed.TextSize = 16

	entrySpeed1 := widget.NewEntry()
	entrySpeed1.SetPlaceHolder("0.00")
	entrySpeed2 := widget.NewEntry()
	entrySpeed2.SetPlaceHolder("0.00")
	entryAccel1 := widget.NewEntry()
	entryAccel1.SetPlaceHolder("0.00")
	entryAccel2 := widget.NewEntry()
	entryAccel2.SetPlaceHolder("0.00")

	directionChoice := []string{"Вперед", "Назад"}
	var selectedDirection1, selectedDirection2 *widget.Select
	selectedDirection1 = widget.NewSelect(directionChoice, func(s string) {
		if s == "Вперед" {
			direction1 = ipk.MotionOnward
			if !separately {
				selectedDirection1.SetSelected("Вперед")
			}
		} else {
			direction1 = ipk.MotionBackwards
			if !separately {
				selectedDirection1.SetSelected("Назад")
			}
		}
	})
	selectedDirection2 = widget.NewSelect(directionChoice, func(s string) {
		if s == "Вперед" {
			direction2 = ipk.MotionOnward
			if !separately {
				selectedDirection2.SetSelected("Вперед")
			}
		} else {
			direction2 = ipk.MotionBackwards
			if !separately {
				selectedDirection2.SetSelected("Назад")
			}
		}
	})
	selectedDirection1.SetSelected("Вперед")
	selectedDirection2.SetSelected("Вперед")

	box1 := container.NewGridWithColumns(
		3,
		dummy, widget.NewLabel("Канал 1"), widget.NewLabel("Канал 2"),
		widget.NewLabel("Скорость (км/ч):"), entrySpeed1, entrySpeed2,
		widget.NewLabel("Ускорение (м/с²):"), entryAccel1, entryAccel2,
		widget.NewLabel("Направление:"), selectedDirection1, selectedDirection2,
		widget.NewLabel("Управление:"), selectbox,
	)

	boxSpeed := container.NewVBox(dummy, textSpeed, box1)

	// Доп. параметры:
	entryDiameter := widget.NewEntry()
	entryDiameter.SetPlaceHolder(fmt.Sprintf("%d", diameter))
	entryNumberTeeth := widget.NewEntry()
	entryNumberTeeth.SetPlaceHolder(fmt.Sprintf("%d", numberTeeth))

	box2 := container.NewGridWithColumns(
		2,
		widget.NewLabel("Кол-во зубьев: "), entryNumberTeeth,
		widget.NewLabel("Диаметр: "), entryDiameter,
	)
	boxAddParameters := container.NewVBox(dummy, box2, dummy)

	// Путь:
	textMileage := canvas.NewText("	Имитация пути: ", color.Black)
	textMileage.TextSize = 16

	entryMileage := widget.NewEntry()
	entryMileage.SetPlaceHolder("20.000")
	buttonMileage := widget.NewButton("Пуск", func() {
		// todo запуск
	})
	labelMileage := widget.NewLabel("") // todo обновлять если запущена проверка
	// byteDistance(mapDataCAN[idDistance])

	box3 := container.NewGridWithColumns(
		3,
		widget.NewLabel("Дистанция (км):"), entryMileage, buttonMileage,
		widget.NewLabel("Текущая (км):"), labelMileage,
	)
	boxMileage := container.NewVBox(dummy, textMileage, box3, dummy)

	// Давление
	textPress := canvas.NewText("	Аналоговые каналы: ", color.Black)
	textPress.TextSize = 16

	entryPressChannel1 := widget.NewEntry()
	entryPressChannel1.SetPlaceHolder("0.00") // todo ограничить 10 атм
	entryPressChannel2 := widget.NewEntry()
	entryPressChannel2.SetPlaceHolder("0.00") // 20 атм
	entryPressChannel3 := widget.NewEntry()
	entryPressChannel3.SetPlaceHolder("0.00") // 20 атм

	box4 := container.NewGridWithColumns(
		2,
		widget.NewLabel("Канал 1 (кгс/см²):"), entryPressChannel1,
		widget.NewLabel("Канал 2 (кгс/см²):"), entryPressChannel2,
		widget.NewLabel("Канал 3 (кгс/см²):"), entryPressChannel3,
	)
	boxPress := container.NewVBox(dummy, textPress, box4, dummy)

	box5 := container.NewVBox(boxSpeed, boxAddParameters)
	boxSpeedAndMileage := container.NewVSplit(box5, boxMileage)

	boxAll := container.NewVSplit(boxSpeedAndMileage, boxPress)
	box := container.NewHBox(boxAll, dummy)

	return container.New(layout.NewGridWrapLayout(fyne.NewSize(450, 850)), box)
}

// коды РЦ (Сигналы ИФ) установить 1 из 7
// Вых.БУ 50В, 10В
func outputSignals() fyne.CanvasObject {
	dummy := widget.NewLabel("")

	labelCode := widget.NewLabel("Коды РЦ:")
	code := []string{"Нет",
		"КЖ 1.6",
		"Ж  1.6",
		"З  1.6",
		"КЖ 1.9",
		"Ж  1.9",
		"З  1.9",
	}
	radio := widget.NewRadioGroup(code, func(s string) {
		fmt.Println(s)
	})
	// radio.Horizontal = true
	boxCode := container.NewVBox(dummy, labelCode, radio)

	labelOut50V := widget.NewLabel("Вых.БУ (50В):")
	checkG := widget.NewCheck("З", nil)            // З		0
	checkY := widget.NewCheck("Ж", nil)            // Ж		1
	checkRY := widget.NewCheck("КЖ", nil)          // КЖ	2
	checkR := widget.NewCheck("К", nil)            // К		3
	checkW := widget.NewCheck("Б", nil)            // Б		4
	checkEPK1 := widget.NewCheck("ЭПК1", nil)      // ЭПК1	5
	checkIF := widget.NewCheck("ИФ", nil)          // ИФ	6
	checkTracktion := widget.NewCheck("Тяга", nil) // Тяга	7
	boxOut50V := container.NewVBox(dummy, labelOut50V, checkG, checkY, checkRY, checkR, checkW, checkEPK1, checkIF, checkTracktion)

	labelOut10V := widget.NewLabel("Вых.БУ (10В):")
	checkLP := widget.NewCheck("ЛП", nil)              // 1
	checkButtonUhod := widget.NewCheck("кн.Уход", nil) // 3
	checkEPK := widget.NewCheck("ЭПК", nil)            // 5
	checkPowerBU := widget.NewCheck("Пит.БУ", nil)     // 7
	checkKeyEPK := widget.NewCheck("Ключ ЭПК", nil)    // 9
	boxOut10V := container.NewVBox(dummy, labelOut10V, checkLP, checkButtonUhod, checkEPK, checkPowerBU, checkKeyEPK)

	boxOut := container.NewVBox(boxOut10V, boxOut50V)
	return container.NewHBox(dummy, boxCode, dummy, boxOut, dummy)
}

// Уставки, входы БУС = считать
func inputSignals() fyne.CanvasObject {
	dummy := widget.NewLabel("")

	labelRelay := widget.NewLabel("Реле уставок:")
	check1 := widget.NewCheck("1", nil)
	check20 := widget.NewCheck("20", nil)
	check80 := widget.NewCheck("80", nil)
	check60 := widget.NewCheck("60", nil)
	check30 := widget.NewCheck("30", nil)
	boxRelay := container.NewVBox(dummy, labelRelay, check1, check20, check80, check60, check30)

	labelBUS := widget.NewLabel("Входы БУС:")
	checkPSS2 := widget.NewCheck("ПСС2", nil)
	checkUhod2 := widget.NewCheck("Уход 2", nil)
	checkPowerEPK := widget.NewCheck("Пит.ЭПК", nil)
	checkPB2 := widget.NewCheck("РБ2", nil)
	checkEVM := widget.NewCheck("ЭВМ", nil)
	boxBUS := container.NewVBox(dummy, labelBUS, checkPSS2, checkUhod2, checkPowerEPK, checkPB2, checkEVM)

	box := container.NewVBox(boxRelay, boxBUS)

	return container.NewHBox(dummy, box, dummy)
}

func top() fyne.CanvasObject {

	powerKPD := binding.NewBool() // питание включается при старте? todo
	powerKPD.Set(true)            // устанавливается в начале
	checkPower := widget.NewCheckWithData("Питание КПД", powerKPD)

	turn := binding.NewBool()
	turn.Set(false)
	checkTurt := widget.NewCheckWithData("Режим обслуживания", turn)

	box := container.NewHBox(checkPower, checkTurt)

	return box
}

//---------------------------------------------------------------------------
