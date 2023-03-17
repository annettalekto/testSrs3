package main

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
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
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/amdf/ipk"
	"github.com/amdf/ixxatvci3"
	"github.com/amdf/ixxatvci3/candev"
	"github.com/shirou/gopsutil/process"
	"github.com/xlab/closer"
)

/*
	Программа «Электронная имитация параметров КПД»
	– для дополнительной (ручной) проверки блоков на заводе (не для потребителя).
	предполагается автоматический кабель
*/

var gForm DescriptionForm
var config configType

var can25 *candev.Device

var bInitCAN bool
var bConnectedIPK bool
var bConnectedCAN bool
var bServiceModeBU4 bool
var bRebootBU bool = true // отслеживаем перезагрузку блока, смену БУ, потерю соединения по CAN

var w fyne.Window

func main() {
	config = getFyneAPP()

	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			fmt.Println("PANIC!")
			os.Exit(1)
		}
	}()

	programAlreadyRunning()

	// Инит
	var b candev.Builder
	var err error

	can25, err = b.Speed(ixxatvci3.Bitrate25kbps).Get()
	if err != nil {
		bInitCAN = false
		fmt.Printf("Ошибка инициализации CAN: %v\n", err)
		err = errors.New("Ошибка инициализации CAN")
	} else {
		bInitCAN = true
		can25.Run()
		defer can25.Stop()
	}

	errConfig := true
	err = initDataBU(config.DeviceVariant)
	if err != nil {
		fmt.Printf("Данные УПП не получены из конфигурационного файла!")
		errConfig = false
	}

	// Форма
	a := app.New()
	w = a.NewWindow(config.ProgramName) // с окнами у fyne проблемы
	// w.Resize(fyne.NewSize(1024, 870))   // если размер менее, то баг при сворачивании
	// w.SetFixedSize(true)                // не использовать без Resize
	// w.SetFullScreen(true)

	ic, _ := fyne.LoadResourceFromPath(config.Icon)
	w.SetIcon(ic)
	w.CenterOnScreen()
	w.SetMaster()

	if config.Theme == "dark" {
		a.Settings().SetTheme(theme.DarkTheme())
	} else {
		a.Settings().SetTheme(fyneLightTheme{})
	}

	menu := fyne.NewMainMenu(
		fyne.NewMenu("Файл",
			fyne.NewMenuItem("Тема", func() { changeTheme() }),
		),

		fyne.NewMenu("Справка",
			fyne.NewMenuItem("Посмотреть справку", func() { aboutHelp() }),
			// fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("О программе", func() { abautProgramm() }),
		),
	)
	w.SetMainMenu(menu)

	go func() { // простите
		sec := time.NewTicker(1 * time.Second)
		for range sec.C {
			for _, item := range menu.Items[0].Items {
				if strings.Contains(item.Label, "Quit") {
					item.Label = "Выход"
					menu.Refresh()
					return
				}
			}
		}
	}()

	// одна общая строка для вывода ошибок, подсказок
	var style fyne.TextStyle
	style.Monospace = true
	gStatusLabel := widget.NewLabel("Статус")
	gStatusLabel.TextStyle = style
	gForm.Status = binding.NewString()
	gStatusLabel.Bind(gForm.Status)

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

	// вывод ошибок полученных при старте программы
	if !errConfig {
		gForm.Status.Set("Не получены данные из файла конфигурации")
	}

	err = initIPK()
	if err != nil {
		fmt.Printf("Ошибка инициализации ИПК: %v\n", err)
		bConnectedIPK = false
	} else {
		bConnectedIPK = true
		fmt.Println("Инициализации ИПК OK")
	}

	/*
		! Start
	*/
	switch {

	case !bInitCAN:
		ErrorDialog("Ошибка CAN", "Выход", "Не удалось произвести инициализацию CAN.\nПодключите CAN адаптер.\nПерезапустите программу.")

	case !bConnectedIPK:
		dialog.ShowCustomError("Ошибка ИПК", "Ок", "Не удалось произвести инициализацию ИПК.\nПроверьте подключение ИПК.", func(b bool) {}, w)
		bTimerReset = false // сообщение выше
		bConnectedIPK = false
		fallthrough

	default:
		activityWindow()
		go threadConnectionCAN()
		go threadConnectionIPK()
		go processScreen()
		go threadShowForm()
		go updateAfterReboot()
	}

	// Делаем Сплиты неподвижным
	go func() {
		sec := time.NewTicker(10 * time.Millisecond)
		for range sec.C {
			if box4.Offset != 0.0 {
				box4.SetOffset(0.0)
			}

			if box1.Offset != 0.5 {
				box1.SetOffset(0.5)
			}

			if box.Offset != 0.5 {
				box.SetOffset(0.5)
			}
		}
	}()

	// запуск формы
	w.SetContent(box)
	w.ShowAndRun()
}

var timeInitIPK *time.Timer

const (
	checkError       = 0
	reinitialization = 1
	connectedIPK     = 2
	timeCheck        = 3
)

const durationInitIPK = 4

var bTimerReset = true

/*
Проверка соединения с ИПК
При отсутвии соединения с модулем переинициализация
*/
func threadConnectionIPK() {

	errF, errB, errA := true, true, true

	var err error
	var stateCheck int

	sec := time.NewTicker(1000 * time.Millisecond)
	for range sec.C {

		switch stateCheck {
		case checkError:
			errA = ipkBox.AnalogDev.Active()
			errB = ipkBox.BinDev.Active()
			errF = ipkBox.FreqDev.Active()

			if !errA || !errB || !errF { // Есть ошибка
				stateCheck = reinitialization
			} else {
				stateCheck = connectedIPK
			}

		case reinitialization:
			if !errA {
				ipkBox.AnalogDev.Close()
			}
			if !errB {
				ipkBox.BinDev.Close()
			}
			if !errF {
				ipkBox.FreqDev.Close()
			}
			err = initIPK()
			if err != nil { // не удалось переинициализировать
				fmt.Println(err)
				ShowMessage(fmt.Sprintf("%v", err), 2)

				if bTimerReset { // запустить таймер отсутвия соединения с ипк
					stateCheck = timeCheck
				} else {
					stateCheck = reinitialization // повторить попытку переинециализации
				}

			} else {
				ShowMessage("Соединение с ИПК установлено", 3)
				stateCheck = connectedIPK
			}

			// даем время на включение ипк, если в течении времени t ипк не будет инициализирован, то выведем сообщение  с просьбой включить ипк
			// баг c плохим ип ипк, ипк отваливается не при выключени 50В, а при включении после, нужно дать время на переинициализацию, если же переинициализации не произошло за заданное время то считаем что ипк не подключен
		case timeCheck:
			bTimerReset = false
			timeInitIPK = time.AfterFunc(durationInitIPK*time.Second, func() {
				dialog.ShowCustomError("Ошибка ИПК", "Ок", "Потеряно соединение с ИПК.\nПодключите ИПК.", func(b bool) {}, w)
				bConnectedIPK = false
			})
			stateCheck = reinitialization

		case connectedIPK:
			bConnectedIPK = true
			bTimerReset = true
			if timeInitIPK != nil { // если запущен таймер - сбросить
				timeInitIPK.Stop()
			}
			stateCheck = checkError

		}
	}
}

func resetInfo() {
	bConnectedCAN = false
	bServiceModeBU4 = false
}

func threadConnectionCAN() {
	errCounter := 0
	for {
		_, err := can25.GetMsgByID(idTimeBU, 500*time.Millisecond) // todo
		if err != nil {
			if errCounter == 5 {
				resetInfo()
				errCounter = 0
			}
			errCounter++

		} else {
			errCounter = 0
			bConnectedCAN = true
		}
	}
}

/*
Блокировка полей ввода, кнопок при отсутвии соединения с БУ или ИПК
*/
func threadShowForm() {

	resetScren()
	sec := time.NewTicker(500 * time.Millisecond)
	for range sec.C {
		if bConnectedCAN {
			if bServiceModeBU4 {
				setStatus("Соединение с БУ установлено. Режим обслуживания.")
			} else if bTurt {
				setStatus("Соединение с БУ установлено. Режим TURT.")
			} else {
				setStatus("Соединение с БУ установлено.")
			}

		} else {
			fmt.Println("Блок БУ не обнаружен")
			setStatus("Проверьте подключение CAN. Включите тумбер ИПК (50В) и переведите БУ в режим поездки")
			// setStatus(fmt.Sprintf("RcvOkCount = %d, RcvErrCount = %d, RcvProcessedActive =  %d, RcvBackgroundNoData = %d", can25.RcvOkCount, can25.RcvErrCount, can25.RcvProcessedActive, can25.RcvBackgroundNoData))

			// fmt.Println("RcvOkCount = ", can25.RcvOkCount)
			// fmt.Println("RcvErrCount = ", can25.RcvErrCount)
			// fmt.Println("RcvProcessedActive = ", can25.RcvProcessedActive)
			// fmt.Println("RcvBackgroundNoData = ", can25.RcvBackgroundNoData)

			bRebootBU = true
		}
		activityWindow()
	}
}

// Обновление данных после перезагрузки
func updateAfterReboot() {
	for {
		if gBU.Variant == BU4 {
			if bConnectedCAN && bRebootBU {
				getVersionBU4()
				bRebootBU = false
			}
		}
		time.Sleep(time.Second)
	}
}

func getVersionBU4() {
	if major, minor, patch, number, err := canGetVersionBU4(); err == nil {
		gBU.VersionBU4 = fmt.Sprintf("Версия %d.%d.%d (в лоции №%d)", major, minor, patch, number)
	} else {
		fmt.Println("%V", err)
	}
}

/*
Ограничения функционала программы при ошибках и в определенных режимах работы
Блокировка полей вводаБ кнопок и прочего функционала
При отсутсвии соединения с ИПК bConnectedIPK и CAN адаптером bConnectedCAN,
в режиме движения bMotion, в режиме поездки bTrip
*/
func activityWindow() {

	if !bConnectedIPK && !bConnectedCAN {

		gForm.CheckPower.Disable()
		gForm.CheckTurt.Disable()

		selectDevice.Disable()

		buttonUPP.Disable()

		buttonMileage.Disable()
		entryMileage.Disable()

		startSpeedButton.Disable()
		entrySpeed1.Disable()
		gForm.EntrySpeed2.Entry.Disable()
		entryAccel1.Disable()
		gForm.EntryAccel2.Entry.Disable()

		startPressButton.Disable()
		entryPress1.Disable()
		gForm.EntryPress2.Entry.Disable()
		gForm.EntryPress3.Entry.Disable()

		radioDirection.Disable()
	}

	if bConnectedIPK && !bConnectedCAN {

		gForm.CheckPower.Enable()

		if gBU.Variant == BU4 {
			gForm.CheckTurt.Disable()
		} else {
			gForm.CheckTurt.Enable()
		}

		// selectDevice.Disable()
		selectDevice.Enable()

		if bTurt {
			buttonUPP.Enable()
		} else {
			buttonUPP.Disable()
		}

		buttonMileage.Disable()
		entryMileage.Disable()

		startSpeedButton.Disable()
		entrySpeed1.Disable()
		gForm.EntrySpeed2.Entry.Disable()
		entryAccel1.Disable()
		gForm.EntryAccel2.Entry.Disable()

		startPressButton.Disable()
		entryPress1.Disable()
		gForm.EntryPress2.Entry.Disable()
		gForm.EntryPress3.Entry.Disable()

		radioDirection.Disable()

	}

	if bConnectedCAN && bConnectedIPK {

		gForm.CheckPower.Enable()

		if gBU.Variant == BU4 {
			if bServiceModeBU4 {
				gForm.CheckTurt.SetChecked(true)
				gForm.CheckTurt.Disable()
			} else {
				gForm.CheckTurt.SetChecked(false)
				if !bTrip && !bMotion {
					gForm.CheckTurt.Enable()
				}
			}
		} else {
			if !bTrip && !bMotion {
				gForm.CheckTurt.Enable()
			} else {
				gForm.CheckTurt.Disable()
			}
		}

		selectDevice.Enable()

		buttonUPP.Enable()

		buttonMileage.Enable()
		entryMileage.Enable()

		entrySpeed1.Enable()
		entryAccel1.Enable()

		if gBU.Variant != BU4 {
			gForm.EntrySpeed2.Entry.Enable()
			gForm.EntryAccel2.Entry.Enable()
			gForm.EntryPress2.Entry.Enable()
			gForm.EntryPress3.Entry.Enable()
		} else {

			if gBU.NumberDUP == 1 {
				gForm.EntrySpeed2.Entry.Disable()
				gForm.EntryAccel2.Entry.Disable()
			} else { // gBU.NumberDUP == 2
				gForm.EntrySpeed2.Entry.Enable()
				gForm.EntryAccel2.Entry.Enable()
			}

			if gBU.NumberDD == 1 {
				gForm.EntryPress2.Entry.Disable()
				gForm.EntryPress3.Entry.Disable()
			} else { //gBU.NumberDD == 2
				gForm.EntryPress2.Entry.Enable()
				gForm.EntryPress3.Entry.Disable()
			}

			// if major, minor, patch, number, err := canGetVersionBU4(); err == nil {
			// 	gBU.VersionBU4 = fmt.Sprintf("Версия %d.%d.%d (в лоции №%d)", major, minor, patch, number)
			// }
		}

		startPressButton.Enable()
		entryPress1.Enable()

		radioDirection.Enable()

		if bTrip {
			buttonMileage.SetText("Стоп")
		} else {
			buttonMileage.SetText("Старт")
		}
		startSpeedButton.Enable()
		buttonMileage.Enable()

		// Имитируем движение, запрет имитации пути
		if bMotion && !bTrip {
			startSpeedButton.SetText("Стоп")
		}
		if !bMotion || bMotion && bTrip {
			startSpeedButton.SetText("Ок")
		}

		if bTurt || bServiceModeBU4 {

			selectDevice.Disable()

			buttonMileage.Disable()
			entryMileage.Disable()

			startSpeedButton.Disable()
			entrySpeed1.Disable()
			gForm.EntrySpeed2.Entry.Disable()
			entryAccel1.Disable()
			gForm.EntryAccel2.Entry.Disable()

			startPressButton.Disable()
			entryPress1.Disable()
			gForm.EntryPress2.Entry.Disable()
			gForm.EntryPress3.Entry.Disable()

			radioDirection.Disable()
		}
	}

}

var textMessage, textStatus string // значение последнего статуса и сообщения
var bShowMessage = false           // признак нового сообщения
var timer *time.Timer

// const showMessageTime = 5

// устанавоиваем текущий статус
func setStatus(message string) {
	textStatus = message
}

/*
Показать сообщение поверх статуса
bShowMessage признак нового сообщения
showMessageTime - сколько с будет отображаться сообщение на экране.
*/
func ShowMessage(message string, messageTime time.Duration) {

	if bShowMessage {
		timer.Stop()
	} else {
		bShowMessage = true
	}

	textMessage = message

	timer = time.AfterFunc(messageTime*time.Second, func() {
		bShowMessage = false // Время отображения сообщения истекло
	})
}

func resetScren() {
	bShowMessage = false
	timer.Stop()
	textMessage = ""
	textStatus = ""
}

// Поток для вывода подсказок и ошибок
func processScreen() {

	sec := time.NewTicker(200 * time.Millisecond)

	for range sec.C {

		// установить
		if bShowMessage {
			gForm.Status.Set(textMessage)
		} else {
			gForm.Status.Set(textStatus)
		}

	}
}

//---------------------------------------------------------------------------//
//								О программе
//---------------------------------------------------------------------------//

// сменить тему
func changeTheme() {

	switch config.Theme {
	case "light":
		fyne.CurrentApp().Settings().SetTheme(theme.DarkTheme())
		config.Theme = "dark"
	case "dark":
		fyne.CurrentApp().Settings().SetTheme(fyneLightTheme{})
		config.Theme = "light"
	default:
		fyne.CurrentApp().Settings().SetTheme(fyneLightTheme{})
		config.Theme = "light"
	}
	writeFyneAPP(config)
}

func aboutHelp() {
	err := exec.Command("cmd", "/C", ".\\help\\index.html").Run()
	if err != nil {
		fmt.Println("Ошибка открытия файла справки")
	}
}

func abautProgramm() {
	w := fyne.CurrentApp().NewWindow("О программе") // CurrentApp!
	w.Resize(fyne.NewSize(450, 160))
	w.SetFixedSize(true)
	w.CenterOnScreen()

	img := canvas.NewImageFromURI(storage.NewFileURI("Logo.png"))
	img.Resize(fyne.NewSize(66, 66))
	img.Move(fyne.NewPos(10, 30))

	l0 := widget.NewLabel(config.ProgramName)
	l0.Move(fyne.NewPos(80, 10))
	l1 := widget.NewLabel(fmt.Sprintf("Версия ПО %s", config.Version))
	l1.Move(fyne.NewPos(80, 40))
	l2 := widget.NewLabel(fmt.Sprintf("Версия сборки %d", config.Build))
	l2.Move(fyne.NewPos(80, 70))
	l3 := widget.NewLabel(fmt.Sprintf("© ПАО «Электромеханика», %s", config.Year))
	l3.Move(fyne.NewPos(80, 100))

	box := container.NewWithoutLayout(img, l0, l1, l2, l3)

	w.SetContent(box)
	w.Show() // ShowAndRun -- panic!
}

func getTitle(str string) *widget.Label {
	var style fyne.TextStyle
	style.Bold = true

	return widget.NewLabelWithStyle(str, fyne.TextAlignCenter, style)
}

//---------------------------------------------------------------------------//
// изменения на главной форме

// DescriptionForm то что изменяется от входных значений
// (при смене уставок в упп нужно менять их на экране или
// скрыть некоторые элементы при смене типа болка)
type DescriptionForm struct {
	Status binding.String // строка (внизу) для ошибок, подсказок и др. инфы

	CheckPower *widget.Check

	RelayY  *widget.Check // уставки скоростей
	RelayRY *widget.Check
	RelayU  *widget.Check

	// Parameters binding.String // параметры имитации скорости (число зубьев и бандаж)

	BoxBUS    *fyne.Container // сигналы БУС (есть только в 3ПВ)
	BoxOut50V *fyne.Container // некоторые сигналы 3ПВ

	// Для БУ-4
	CheckTurt   *widget.Check // turt нет, есть режим обслуживания (уст-ся через can)
	EntrySpeed2 *numericalEntry
	EntryAccel2 *numericalEntry
	EntryPress2 *numericalEntry
	EntryPress3 *numericalEntry
	BoxOut10V   *fyne.Container
	Radio       *widget.RadioGroup
}

func changeFormBU4() {

	gForm.CheckTurt.Text = "Режим обслуживания"
	gForm.CheckTurt.Refresh()

	gForm.BoxBUS.Hide()
	gForm.BoxOut50V.Hide()
	gForm.BoxOut10V.Hide()
	gForm.Radio.Hide()
}

// обновить данные на форме если было изменено значение УПП или выбран новый блок
func refreshForm() (err error) {

	refreshDataIPK()

	// gForm.Parameters.Set(fmt.Sprintf("Число зубьев:	 	%d, 	диаметр бандажа:	 %d мм", gBU.NumberTeeth, gBU.BandageDiameter))
	gForm.RelayY.Text = fmt.Sprintf("%d", gBU.RelayY)
	gForm.RelayRY.Text = fmt.Sprintf("%d", gBU.RelayRY)
	gForm.RelayU.Text = fmt.Sprintf("%d", gBU.RelayU)
	gForm.RelayY.Refresh()
	gForm.RelayRY.Refresh()
	gForm.RelayU.Refresh()

	if gBU.Variant != BU4 {
		gForm.CheckTurt.Text = "TURT"
		gForm.CheckTurt.Refresh()
		gForm.BoxOut10V.Show()
		gForm.Radio.Show()
	}

	switch gBU.Variant {
	case BU3P:
		gForm.CheckPower.Hide()
		gForm.BoxBUS.Hide()
		gForm.BoxOut50V.Hide()
	case BU3PA:
		gForm.CheckPower.Show()
		gForm.BoxBUS.Hide()
		gForm.BoxOut50V.Hide()
	case BU3PV:
		gForm.CheckPower.Show()
		gForm.BoxBUS.Show()
		gForm.BoxOut50V.Show()
	case BU4:
		gForm.CheckPower.Show()
		changeFormBU4()
	}

	return

}

//---------------------------------------------------------------------------//
// 								Данные CAN
//---------------------------------------------------------------------------//

var mu sync.Mutex
var gDataCAN = make(map[uint32][8]byte)
var gBuErrors []int

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

func getDataCAN() map[uint32][8]byte { // переименовать todo
	mapDataCAN := make(map[uint32][8]byte)

	mu.Lock()
	mapDataCAN = gDataCAN
	mu.Unlock()

	return mapDataCAN
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
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if i < len(data) {
				o.(*widget.Label).SetText(data[i])
			}
		})

	list.OnSelected = func(id widget.ListItemID) {
		if strings.HasPrefix(data[id], "H") {
			sCodeError := strings.TrimPrefix(data[id], "H")
			sErrorDescription := getErrorDescription(sCodeError)
			ShowMessage(fmt.Sprintf("H%s: %s", sCodeError, sErrorDescription), 4)
			list.Unselect(id) // сбросить выделение пункта, чтобы сново можно было по нему тыкать и получать подсказку
		} else {
			ShowMessage("", 2)
		}
	}

	// обновление данных
	go func() {
		for {
			data = nil
			mapDataCAN := getDataCAN()

			if bConnectedCAN {
				t := byteToTimeBU(mapDataCAN[idTimeBU])
				data = append(data, fmt.Sprintf("Время БУ:  %s", t.Format("02.01.2006 15:04")))
			} else {
				data = append(data, "Время БУ: -")
			}

			if gBU.Variant == BU4 {
				if bConnectedCAN {
					data = append(data, gBU.VersionBU4)
				} else {
					data = append(data, "Версия -.-.- (в лоции №-)")
				}
			} else {
				if bytes, ok := mapDataCAN[idDigitalInd]; ok {
					str := byteToDigitalIndicator(bytes)
					data = append(data, fmt.Sprintf("%s %s", "Осн. инд.:", str))
				} else {
					data = append(data, fmt.Sprintf("%s —", "Осн. инд.:"))
				}

				if bytes, ok := mapDataCAN[idAddInd]; ok {
					str := byteToAddIndicator(bytes)
					data = append(data, fmt.Sprintf("%s %s", "Доп. инд.:", str))
				} else {
					data = append(data, fmt.Sprintf("%s —", "Доп. инд.:"))
				}
			}

			data = append(data, " ")

			if bytes, ok := mapDataCAN[idSpeed1]; ok {
				enterSpeed1, _ := currentSpeed1.Get()
				enterSpeed2, _ := currentSpeed2.Get()
				s1, _ := strconv.ParseFloat(enterSpeed1, 64)
				s2, _ := strconv.ParseFloat(enterSpeed2, 64)
				if s1 > 400 || s2 > 400 { // БУ присылает неверные данные, если скорость более 400 км/ч, отслеживаем по введенным данным
					data = append(data, fmt.Sprintf("%-22s Ошибка", "Скорость 1 каб.(км/ч):"))
				} else {
					data = append(data, fmt.Sprintf("%-22s %.1f", "Скорость 1 каб.(км/ч):", byteToSpeed(bytes)))
				}
			} else {
				data = append(data, fmt.Sprintf("%-22s —", "Скорость 1 каб.(км/ч):"))
			}
			if bytes, ok := mapDataCAN[idSpeed2]; ok {
				// enterSpeed1, _ := strconv.Atoi(entrySpeed1.Entry.Text)
				// enterSpeed2, _ := strconv.Atoi(gForm.EntrySpeed2.Entry.Text)
				enterSpeed1, _ := currentSpeed1.Get()
				enterSpeed2, _ := currentSpeed2.Get()
				s1, _ := strconv.ParseFloat(enterSpeed1, 64)
				s2, _ := strconv.ParseFloat(enterSpeed2, 64)
				if s1 > 400 || s2 > 400 {
					data = append(data, fmt.Sprintf("%-22s Ошибка", "Скорость 2 каб.(км/ч):"))
				} else {
					data = append(data, fmt.Sprintf("%-22s %.1f", "Скорость 2 каб.(км/ч):", byteToSpeed(bytes)))
				}
			} else {
				data = append(data, fmt.Sprintf("%-22s —", "Скорость 2 каб.(км/ч):"))
			}

			if bytes, ok := mapDataCAN[idPressure]; ok {
				tm, tc, gr := byteToPressure(bytes)

				// БУ-3П на значение давления 0 присылвает в ответ 0.1
				if gBU.Variant == BU3P {
					if entryPress1.Entry.Text == "0.00" {
						tm = 0.0
					}
					if gForm.EntryPress2.Text == "0.00" {
						tc = 0.0
					}
					if gForm.EntryPress3.Text == "0.00" {
						gr = 0.0
					}
				}

				data = append(data, fmt.Sprintf("%-22s %.1f", "Давление ТМ (кг/см²):", tm))
				data = append(data, fmt.Sprintf("%-22s %.1f", "Давление ТЦ (кг/см²):", tc))
				if gBU.Variant != BU4 {
					data = append(data, fmt.Sprintf("%-22s %.1f", "Давление ГР (кг/см²):", gr))
				}
			} else {
				data = append(data, fmt.Sprintf("%-22s —", "Давление ТМ (кг/см²):"))
				data = append(data, fmt.Sprintf("%-22s —", "Давление ТЦ (кг/см²):"))
				if gBU.Variant != BU4 {
					data = append(data, fmt.Sprintf("%-22s —", "Давление ГР (кг/см²):"))
				}
			}

			if bytes, ok := mapDataCAN[idDistance]; ok {
				u := byteDistance(bytes)
				data = append(data, fmt.Sprintf("%-22s %d", "Дистанция (м):", u))
			} else {
				data = append(data, fmt.Sprintf("%-22s —", "Дистанция (м):"))
			}

			data = append(data, " ") // просто отступ

			if gBU.Variant != BU4 {
				if bytes, ok := mapDataCAN[idALS]; ok {
					_, str := byteToALS(bytes)
					data = append(data, fmt.Sprintf("%-16s %s", "АЛС:", str))
					if (bytes[0] & 0x40) == 0x40 {
						str = "1"
					} else {
						str = "0"
					}
					data = append(data, fmt.Sprintf("%-16s %s", "Kлюч ЭПК 1 каб:", str))
					if (bytes[0] & 0x80) == 0x80 {
						str = "1"
					} else {
						str = "0"
					}
					data = append(data, fmt.Sprintf("%-16s %s", "Kлюч ЭПК 2 каб:", str))
					if (bytes[3] & 0x20) == 0x20 {
						str = "2"
					} else {
						str = "1"
					}
					data = append(data, fmt.Sprintf("%-16s %s", "Активна каб.:", str))
					if (bytes[5] & 0x20) == 0x20 {
						str = "1"
					} else {
						str = "0"
					}
					data = append(data, fmt.Sprintf("%-16s %s", "Cостояние ЭПК:", str))
					if (bytes[6] & 0x20) == 0x20 {
						str = "1"
					} else {
						str = "0"
					}
					data = append(data, fmt.Sprintf("%-16s %s", "Активность САУТ:", str))
				} else {
					data = append(data, fmt.Sprintf("%-16s —", "АЛС:"))
					data = append(data, fmt.Sprintf("%-16s —", "Kлюч ЭПК 1 каб:"))
					data = append(data, fmt.Sprintf("%-16s —", "Kлюч ЭПК 2 каб:"))
					data = append(data, fmt.Sprintf("%-16s —", "Активна каб.:"))
					data = append(data, fmt.Sprintf("%-16s —", "Cостояние ЭПК:"))
					data = append(data, fmt.Sprintf("%-16s —", "Активность САУТ:"))
				}

				if bytes, ok := mapDataCAN[idCodeIF]; ok {
					_, _, _, str := byteToCodeIF(bytes)
					data = append(data, fmt.Sprintf("%-16s %s", "Сигнал ИФ:", str))
				} else {
					data = append(data, fmt.Sprintf("%-16s —", "Сигнал ИФ:"))
				}
			}

			if bytes, ok := mapDataCAN[idBin]; ok {
				str := ""
				if (bytes[1] & 0x01) == 0x01 {
					str = "установлено"
				} else {
					str = "сброшено"
				}
				data = append(data, fmt.Sprintf("%-16s %s", "Движение вперёд:", str))

				if (bytes[1] & 0x02) == 0x02 {
					str = "установлено"
				} else {
					str = "сброшено"
				}
				data = append(data, fmt.Sprintf("%-16s %s", "Движение назад:", str))

				if (bytes[1] & 0x10) == 0x10 {
					str = "установлен"
				} else {
					str = "сброшен"
				}
				data = append(data, fmt.Sprintf("%-16s %s", "Сигнал Тяга:", str))

				if gBU.Variant != BU4 {
					if (bytes[2] & 0x08) == 0x08 {
						str = "1"
					} else {
						str = "0"
					}
					data = append(data, fmt.Sprintf("%-16s %s", "Кран ЭПК 1 каб.:", str))
					if (bytes[2] & 0x10) == 0x10 {
						str = "1"
					} else {
						str = "0"
					}
					data = append(data, fmt.Sprintf("%-16s %s", "Кран ЭПК 2 каб.:", str))
				}
			} else {
				data = append(data, fmt.Sprintf("%-16s —", "Движение вперёд:"))
				data = append(data, fmt.Sprintf("%-16s —", "Движение назад:"))
				data = append(data, fmt.Sprintf("%-16s —", "Сигнал Тяга:"))
				if gBU.Variant != BU4 {
					data = append(data, fmt.Sprintf("%-16s —", "Кран ЭПК 1 каб.:"))
					data = append(data, fmt.Sprintf("%-16s —", "Кран ЭПК 2 каб.:"))
				}
			}

			data = append(data, " ")

			if len(gBuErrors) > 0 {
				buErrors := append(gBuErrors)
				gBuErrors = nil

				if len(buErrors) > 0 {
					data = append(data, "Ошибки:")

					sort.Ints(buErrors)
					for _, x := range buErrors {
						if x != 0 {
							data = append(data, fmt.Sprintf("H%d", x))
						}
					}
				}
			}

			list.Refresh()
			time.Sleep(1 * time.Second)
		}
	}()

	// задаем минимальную ширину бордера
	// labelDummy := widget.NewLabel("                                                                    ")

	customDummy := canvas.NewText("____________________________________________________________________ ", color.White) // кастомный отступ
	customDummy.TextSize = 10

	box := container.NewBorder(getTitle("Данные CAN:"), customDummy, nil, nil, list)

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

var ch <-chan candev.Message
var idx uint

func getCAN() {

	go func() {
		// threadActivityOk <- 1
		stop := false
		ch, idx = can25.GetMsgChannelCopy()
		// defer can25.CloseMsgChannelCopy(idx)

		for !stop {
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

					if msg.ID == idBu3pSysInfo {
						receiveMsgBu3pSysInfo.Data = msg.Data
					}

					if msg.ID == idSysInfo {
						msgSysInfo.Data = msg.Data
					}

					if msg.ID == BU4_SYS_INFO {
						if msg.Data[0] == SERVICE_MODE {
							if msg.Data[1] == 1 {
								// logInfo = fmt.Sprintf("Блок перешел в режим обслуживания.")
								bServiceModeBU4 = true
							} else {
								bServiceModeBU4 = false
							}
						}
					}

					if msg.ID == SYS_DATA {
						msgSoftVersionBU4.Data = msg.Data
					}

					if msg.ID == idTimeBU { // todo можно сделать прием не так часто
						msgTime.Data = msg.Data
					}

					mu.Unlock()

				}
			default:
			}
			runtime.Gosched()
		}
	}()

}

//---------------------------------------------------------------------------//
// 						ИНТЕРФЕЙС: ФАС, ФЧС
//---------------------------------------------------------------------------//

var bMotion = false
var bTrip = false
var bTurt = false

func newSpecialEntry(initValue string) (e *numericalEntry) {
	e = newNumericalEntry()
	e.Entry.Wrapping = fyne.TextTruncate
	e.Entry.TextStyle.Monospace = true
	e.Entry.SetText(initValue)
	return e
}

var entrySpeed1 *numericalEntry
var entryAccel1 *numericalEntry
var radioDirection *widget.RadioGroup
var entryMileage *numericalEntry
var entryPress1 *numericalEntry

var startSpeedButton *widget.Button
var buttonMileage *widget.Button
var startPressButton *widget.Button

var currentSpeed1 binding.String
var currentSpeed2 binding.String

// Скорость, дистанция, давление
func speed() fyne.CanvasObject {
	var err error

	// ------------------------- box 1 ----------------------------

	separately := binding.NewBool() // cовместное или раздельное управление
	direction := uint8(ipk.MotionOnward)
	speed1, speed2, accel1, accel2 := float64(0), float64(0), float64(0), float64(0)
	speedLimit := 1000
	dummy := widget.NewLabel("")

	customDummy := canvas.NewText(" ", color.White) // кастомный отступ
	customDummy.TextSize = 4

	stopSpeed := func() {
		sp.SetSpeed(0, 0)
		sp.SetAcceleration(0, 0)
		entrySpeed1.Entry.SetText(fmt.Sprintf("%.1f", 0.0))
		gForm.EntrySpeed2.Entry.SetText(fmt.Sprintf("%.1f", 0.0))
		entryAccel1.Entry.SetText(fmt.Sprintf("%.2f", 0.00))
		gForm.EntryAccel2.Entry.SetText(fmt.Sprintf("%.2f", 0.00))
		time.Sleep(1 * time.Second)
	}

	// обработка скорости
	entrySpeed1 = newSpecialEntry("0.0")
	gForm.EntrySpeed2 = newSpecialEntry("0.0")

	entrySpeed1.Entry.OnChanged = func(str string) {
		if str == "" {
			return
		}
		str = strings.ReplaceAll(str, ",", ".")
		if speed1, err = strconv.ParseFloat(str, 64); err != nil {
			fmt.Printf("Ошибка перевода строки в число (скорость 1)\n")
			ShowMessage("Ошибка в поле ввода «Скорость 1»", 4)
			return
		}
		if sep, _ := separately.Get(); !sep {
			speed2 = speed1
			gForm.EntrySpeed2.SetText(str)
		}
		if speed1 > float64(speedLimit) {
			ShowMessage(fmt.Sprintf("Скорость 1: максимум %d км/ч", speedLimit), 4)
		}
		if speed2 > float64(speedLimit) {
			ShowMessage(fmt.Sprintf("Скорость 2: максимум %d км/ч", speedLimit), 4)
		}
	}
	entrySpeed1.Entry.OnSubmitted = func(str string) {
		selectAll()
		if speed1 > float64(speedLimit) || speed2 > float64(speedLimit) {
			ShowMessage("Ошибка установки скорости", 4)
			return
		}
		if err = sp.SetSpeed(speed1, speed2); err != nil {
			fmt.Printf("Ошибка установки скорости")
			ShowMessage("Ошибка установки скорости", 4)
			return
		}
		ShowMessage(" ", 2)
		if strings.Contains(str, ".") {
			entrySpeed1.Entry.SetText(fmt.Sprintf("%.1f", speed1))
			gForm.EntrySpeed2.Entry.SetText(fmt.Sprintf("%.1f", speed2))
		} else {
			entrySpeed1.Entry.SetText(fmt.Sprintf("%.0f", speed1))
			gForm.EntrySpeed2.Entry.SetText(fmt.Sprintf("%.0f", speed2))
		}
		fmt.Printf("Скорость: %.1f %.1f км/ч (%v)\n", speed1, speed2, err)
		if speed1 != 0 || speed2 != 0 {
			bMotion = true
		} else if speed1 == 0 && speed2 == 0 && accel1 == 0 && accel2 == 0 {
			bMotion = false
			stopSpeed()
		}
	}

	gForm.EntrySpeed2.Entry.OnChanged = func(str string) {
		if str == "" {
			return
		}
		str = strings.ReplaceAll(str, ",", ".")
		if speed2, err = strconv.ParseFloat(str, 64); err != nil {
			fmt.Printf("Ошибка перевода строки в число (скорость 2)\n")
			ShowMessage("Ошибка в поле ввода «Скорость 2»", 4)
			return
		}
		if sep, _ := separately.Get(); !sep {
			speed1 = speed2
			entrySpeed1.Entry.SetText(str)
		}
		if speed1 > float64(speedLimit) {
			ShowMessage(fmt.Sprintf("Скорость 1: максимум %d км/ч", speedLimit), 4)
		}
		if speed2 > float64(speedLimit) {
			ShowMessage(fmt.Sprintf("Скорость 2: максимум %d км/ч", speedLimit), 4)
		}
	}
	gForm.EntrySpeed2.Entry.OnSubmitted = func(str string) {
		selectAll()
		if speed1 > float64(speedLimit) || speed2 > float64(speedLimit) {
			ShowMessage("Ошибка установки скорости", 4)
		}
		if err = sp.SetSpeed(speed1, speed2); err != nil {
			fmt.Printf("Ошибка установки скорости")
			ShowMessage("Ошибка установки скорости", 4)
			return
		}
		ShowMessage(" ", 2)
		if strings.Contains(str, ".") {
			entrySpeed1.Entry.SetText(fmt.Sprintf("%.1f", speed1))
			gForm.EntrySpeed2.Entry.SetText(fmt.Sprintf("%.1f", speed2))
		} else {
			entrySpeed1.Entry.SetText(fmt.Sprintf("%.0f", speed1))
			gForm.EntrySpeed2.Entry.SetText(fmt.Sprintf("%.0f", speed2))
		}
		fmt.Printf("Скорость: %.1f %.1f  (%v)\n", speed1, speed2, err)
		if speed1 != 0 || speed2 != 0 {
			bMotion = true
		} else if speed1 == 0 && speed2 == 0 && accel1 == 0 && accel2 == 0 {
			bMotion = false
			stopSpeed()
		}
	}

	// обработка ускорения
	entryAccel1 = newSpecialEntry("0.00")
	gForm.EntryAccel2 = newSpecialEntry("0.00")
	accelLimit := float64(100)

	entryAccel1.Entry.OnChanged = func(str string) {
		if str == "" {
			return
		}
		str = strings.ReplaceAll(str, ",", ".")
		if accel1, err = strconv.ParseFloat(str, 64); err != nil {
			fmt.Printf("Ошибка перевода строки в число (ускорение 1)\n")
			ShowMessage("Ошибка в поле ввода «Ускорение 1»", 4)
			return
		}
		if sep, _ := separately.Get(); !sep {
			accel2 = accel1
			gForm.EntryAccel2.Entry.SetText(str)
		}
		if accel1 > accelLimit {
			ShowMessage(fmt.Sprintf("Ускорение 1: максимум %.0f км/ч", accelLimit), 4)
		}
		if accel2 > accelLimit {
			ShowMessage(fmt.Sprintf("Ускорение 2: максимум %.0f км/ч", accelLimit), 4)
		}
	}
	entryAccel1.Entry.OnSubmitted = func(str string) {
		selectAll()
		if accel1 > accelLimit || accel2 > accelLimit {
			ShowMessage("Ошибка установки ускорения", 4)
			return
		}
		if err = sp.SetAcceleration(accel1*100, accel2*100); err != nil {
			fmt.Printf("Ошибка установки ускорения\n")
			ShowMessage("Ошибка установки ускорения", 4)
			return
		}
		ShowMessage(" ", 2)
		entryAccel1.Entry.SetText(fmt.Sprintf("%.2f", accel1))
		gForm.EntryAccel2.Entry.SetText(fmt.Sprintf("%.2f", accel2))
		fmt.Printf("Ускорение: %.1f %.1f м/с2 (%v)\n", accel1, accel2, err)

		if accel1 != 0 || accel2 != 0 {
			bMotion = true
		} else if speed1 == 0 && speed2 == 0 && accel1 == 0 && accel2 == 0 {
			bMotion = false
			stopSpeed()
		}
	}

	gForm.EntryAccel2.Entry.OnChanged = func(str string) {
		if str == "" {
			return
		}
		str = strings.ReplaceAll(str, ",", ".")
		if accel2, err = strconv.ParseFloat(str, 64); err != nil {
			fmt.Printf("Ошибка перевода строки в число (ускорение 2)\n")
			ShowMessage("Ошибка в поле ввода «Ускорение 2»", 4)
			return
		}
		if sep, _ := separately.Get(); !sep {
			accel1 = accel2
			entryAccel1.Entry.SetText(str)
		}
		if accel1 > accelLimit {
			ShowMessage(fmt.Sprintf("Ускорение 1: максимум %.0f км/ч", accelLimit), 4)
		}
		if accel2 > accelLimit {
			ShowMessage(fmt.Sprintf("Ускорение 2: максимум %.0f км/ч", accelLimit), 4)
		}
	}
	gForm.EntryAccel2.Entry.OnSubmitted = func(str string) {
		selectAll()
		if accel1 > accelLimit || accel2 > accelLimit {
			ShowMessage("Ошибка установки ускорения", 4)
			return
		}
		if err = sp.SetAcceleration(accel1*100, accel2*100); err != nil {
			fmt.Printf("Ошибка установки ускорения\n")
			ShowMessage("Ошибка установки ускорения", 4)
			return
		}
		ShowMessage(" ", 2)
		entryAccel1.Entry.SetText(fmt.Sprintf("%.2f", accel1))
		gForm.EntryAccel2.Entry.SetText(fmt.Sprintf("%.2f", accel2))
		fmt.Printf("Ускорение: %.1f %.1f м/с2 (%v)\n", accel1, accel2, err)
		if accel1 != 0 || accel2 != 0 {
			bMotion = true
		} else if speed1 == 0 && speed2 == 0 && accel1 == 0 && accel2 == 0 {
			bMotion = false
			stopSpeed()
		}
	}

	// тестировщик очень хочет тут кнопку
	startSpeedButton = widget.NewButton("Ок", func() {

		if bMotion && !bTrip { // сбросить скорость и ускорение если в движении, но не в поездке на дистанцию
			stopSpeed()
			bMotion = false
		}
		if !bMotion || bTrip { // если нет движения установить значения из полей ввода если хотя бы одно не нулевое
			if entrySpeed1.Entry.Text != "0.0" || gForm.EntrySpeed2.Entry.Text != "0.0" ||
				entryAccel1.Entry.Text != "0.00" || gForm.EntryAccel2.Entry.Text != "0.00" {
				entrySpeed1.Entry.OnSubmitted(entrySpeed1.Entry.Text)
				gForm.EntrySpeed2.Entry.OnSubmitted(gForm.EntrySpeed2.Entry.Text)
				entryAccel1.Entry.OnSubmitted(entryAccel1.Entry.Text)
				gForm.EntryAccel2.Entry.OnSubmitted(gForm.EntryAccel2.Entry.Text)
				bMotion = true
			}
		}

	})

	// обработка направления
	directionChoice := []string{"Вперёд", "Назад"}

	radioDirection = widget.NewRadioGroup(directionChoice, func(s string) {
		if s == "Вперёд" {
			direction = ipk.MotionOnward
		} else {
			direction = ipk.MotionBackwards
		}

		if err = sp.SetMotion(direction); err != nil { // todo должно быть два напревления
			ShowMessage("Ошибка установки направления движения", 4)
			return
		}
		fmt.Printf("Направление: %s\n", s)
		// ShowMessage(" ")
	})
	radioDirection.Horizontal = true
	radioDirection.SetSelected("Вперёд")

	separatlyCheck := widget.NewCheckWithData("Раздельное управление", separately)

	currentSpeed1 = binding.NewString()
	currentSpeed1.Set("0")
	currentSpeed2 = binding.NewString()
	currentSpeed2.Set("0")

	labelSpeedCurrent1 := widget.NewLabel("0.0")
	labelSpeedCurrent1.Bind(currentSpeed1)
	labelSpeedCurrent2 := widget.NewLabel("0.0")
	labelSpeedCurrent2.Bind(currentSpeed2)

	go currentSpeed()

	box11 := container.NewGridWithColumns(
		3,
		dummy, widget.NewLabel("Канал 1"), widget.NewLabel("Канал 2"),
	)

	box12 := container.NewGridWithColumns(
		3,
		widget.NewLabel("Текущая\ncкорость (км/ч):"), labelSpeedCurrent1, labelSpeedCurrent2,
	)

	box13 := container.NewGridWithColumns(
		3,
		widget.NewLabel("Скорость (км/ч):"), entrySpeed1, gForm.EntrySpeed2,
		widget.NewLabel("Ускорение (м/с²):"), entryAccel1, gForm.EntryAccel2,
		dummy, dummy, startSpeedButton,
	)

	boxSpeed := container.NewVBox(getTitle("Имитация движения:"), box11, box12, box13, separatlyCheck, radioDirection /*, labelParameters*/)

	// Отображаем эталонную скорость

	// ------------------------- box 2 ----------------------------

	// distanceCheck := false
	startDistance, setDistance := uint32(0), uint32(0)
	currentDistance := binding.NewString()
	currentDistance.Set("0")
	distanceLimit := uint32(1000000)

	// обработка пути
	entryMileage = newSpecialEntry("0")
	entryMileage.Entry.OnChanged = func(str string) {
		if str == "" {
			return
		}
		if strings.Contains(str, ".") { // запятая?
			ShowMessage("Ошибка в поле ввода «Дистанция»: введите целое число", 4)
			return
		}
		d, err := strconv.Atoi(str)
		if err != nil {
			setDistance = 0
			fmt.Printf("Ошибка перевода строки в число (путь)\n")
			ShowMessage("Ошибка в поле ввода «Дистанция»", 4)
			return
		}
		setDistance = uint32(d)

		if setDistance > distanceLimit {
			ShowMessage(fmt.Sprintf("Дистанция: максимум %d м", distanceLimit), 4)
		}
	}

	startMileage := func() bool {
		currentDistance.Set("0")

		if setDistance == 0 {
			ShowMessage("Ошибка установки пути", 4)
			return false
		}
		if setDistance > distanceLimit {
			ShowMessage("Ошибка установки пути", 4)
			return false
		}
		if err = sp.SetLimitWay(setDistance); err != nil {
			fmt.Printf("Ошибка установки пути\n")
			ShowMessage("Ошибка установки пути", 4)
			return false
		}
		time.Sleep(1 * time.Second) // не успевает сбросится счетчик
		if startDistance, _, err = sp.GetWay(); err != nil {
			fmt.Printf("Ошибка: не получено значение пути с ИПК\n")
			ShowMessage("Ошибка: не получено значение пути с ИПК", 4)
			return false
		}
		startDistance = 0
		ShowMessage(" ", 2)
		fmt.Printf("Путь: %d м (%v)\n", setDistance, err)

		entryMileage.Entry.SetText(fmt.Sprintf("%d", setDistance))

		if accel1 == 0 && accel2 == 0 && speed1 == 0 && speed2 == 0 {
			ShowMessage("Ошибка: не получено значение скорости или ускорения", 4)
			return false
		}

		// скорость должны установить сами в поле ввода скорости
		if !bMotion {
			entrySpeed1.Entry.OnSubmitted(entrySpeed1.Entry.Text)
			gForm.EntrySpeed2.Entry.OnSubmitted(gForm.EntrySpeed2.Entry.Text)
			entryAccel1.Entry.OnSubmitted(entryAccel1.Entry.Text)
			gForm.EntryAccel2.Entry.OnSubmitted(gForm.EntryAccel2.Entry.Text)
		}

		bTrip = true
		return true
	}

	stopMileage := func() {
		setDistance = 0
		sp.SetSpeed(0, 0)
		sp.SetAcceleration(0, 0)
		entryMileage.Entry.SetText(fmt.Sprintf("%d", setDistance))
		entrySpeed1.Entry.SetText(fmt.Sprintf("%.1f", 0.0))
		gForm.EntrySpeed2.Entry.SetText(fmt.Sprintf("%.1f", 0.0))
		entryAccel1.Entry.SetText(fmt.Sprintf("%.2f", 0.00))
		gForm.EntryAccel2.Entry.SetText(fmt.Sprintf("%.2f", 0.00))
		time.Sleep(1 * time.Second)
		bTrip = false
		bMotion = false
		// startMileage()
		// currentDistance.Set("0")
	}

	// запуск по нажатию кнопки

	buttonMileage = widget.NewButton("Старт", func() {
		if !bTrip {
			startMileage()
		} else {
			stopMileage()
		}
	})
	labelMileage := widget.NewLabel("0")
	labelMileage.Bind(currentDistance)

	// запуск по нажатию Enter
	entryMileage.Entry.OnSubmitted = func(str string) {
		if !bTrip {
			startMileage()
		} else {
			ShowMessage("Завершите поездку", 3)
		}
	}

	box2 := container.NewGridWithColumns(
		3,
		widget.NewLabel("Дистанция:"), entryMileage, buttonMileage,
		widget.NewLabel("Текущая:"), labelMileage,
	)
	boxMileage := container.NewVBox(getTitle("Имитация пути (м):"), box2)

	go func() {
		for {
			if bTrip {
				m, _, err := sp.GetWay()
				if err != nil {
					fmt.Printf("Не получено значение пути с ИПК\n")
					ShowMessage("Ошибка: не получено значение пути с ИПК", 4)
					break
				}
				// else {
				// 	ShowMessage(" ", 2)
				// }
				fmt.Println(m)
				m -= startDistance
				currentDistance.Set(fmt.Sprintf("%d", m))

				if m >= setDistance {
					fmt.Println("Дистанция пройдена")
					ShowMessage("Дистанция пройдена", 4)

					stopMileage()
				}
			}
			time.Sleep(time.Second)
		}
	}()

	// ------------------------- box 3 ----------------------------

	var press1, press2, press3 float64
	limit1, limit2, limit3 := 10., gBU.PressureLimit, 10.

	// обработка давления
	entryPress1 = newSpecialEntry("0.00")
	entryPress1.Entry.OnChanged = func(str string) {
		if str == "" {
			return
		}
		str = strings.ReplaceAll(str, ",", ".")
		if press1, err = strconv.ParseFloat(str, 64); err != nil {
			fmt.Printf("Ошибка перевода строки в число (давление 1)\n")
			ShowMessage("Ошибка в поле ввода «Давление 1»", 4)
			return
		}
		if press1 > limit1 {
			ShowMessage(fmt.Sprintf("Давление 1: максимум %.0f кгс/см2", limit1), 4)
		}
		if press1 < 0 {
			ShowMessage(fmt.Sprintf("Давление должно быть положительным"), 4)
		}
	}
	entryPress1.Entry.OnSubmitted = func(str string) {
		selectAll()
		if gBU.Variant != BU4 {
			err = channel1.Set(math.Abs(press1))
		} else {
			err = channel1BU4.Set(math.Abs(press1))
		}
		if err != nil {
			fmt.Printf("Ошибка установки давления 1\n")
			ShowMessage("Ошибка установки давления 1", 4)
			return
		}
		ShowMessage(" ", 2)
		fmt.Printf("Давление 1: %.2f кгс/см2 (%v)\n", math.Abs(press1), err)
		entryPress1.Entry.SetText(fmt.Sprintf("%.2f", math.Abs(press1)))
	}

	gForm.EntryPress2 = newSpecialEntry("0.00")
	gForm.EntryPress2.Entry.OnChanged = func(str string) {
		if str == "" {
			return
		}
		str = strings.ReplaceAll(str, ",", ".")
		press2, err = strconv.ParseFloat(str, 64)
		if err != nil {
			fmt.Printf("Ошибка перевода строки в число (давление 2)\n")
			ShowMessage("Ошибка в поле ввода «Давление 2»", 4)
			return
		}
		limit2 = gBU.PressureLimit
		if press2 > limit2 {
			ShowMessage(fmt.Sprintf("Давление 2: максимум %.0f кгс/см2", limit2), 4)
		}
		if press2 < 0 {
			ShowMessage(fmt.Sprintf("Давление должно быть положительным"), 4)
		}
	}
	gForm.EntryPress2.Entry.OnSubmitted = func(str string) {
		selectAll()
		if gBU.Variant != BU4 {
			err = channel2.Set(math.Abs(press2))
		} else {
			err = channel2BU4.Set(math.Abs(press2))
		}
		if err != nil {
			fmt.Printf("Ошибка установки давления 2\n")
			ShowMessage("Ошибка установки давления 2", 4)
			return
		}
		ShowMessage(" ", 2)
		fmt.Printf("Давление 2: %.2f кгс/см2 (%v)\n", math.Abs(press2), err)
		gForm.EntryPress2.Entry.SetText(fmt.Sprintf("%.2f", math.Abs(press2)))
	}

	gForm.EntryPress3 = newSpecialEntry("0.00")
	gForm.EntryPress3.Entry.OnChanged = func(str string) {
		if str == "" {
			return
		}
		str = strings.ReplaceAll(str, ",", ".")
		press3, err = strconv.ParseFloat(str, 64)
		if err != nil {
			fmt.Printf("Ошибка перевода строки в число (давление 3)\n")
			ShowMessage("Ошибка в поле ввода «Давление 3»", 4)
			return
		}
		if press3 > limit3 {
			ShowMessage(fmt.Sprintf("Давление 3: максимум %.0f кгс/см2", limit3), 4)
		}
		if press3 < 0 {
			ShowMessage(fmt.Sprintf("Давление должно быть положительным"), 4)
		}
	}
	gForm.EntryPress3.Entry.OnSubmitted = func(str string) {
		selectAll()
		if err = channel3.Set(math.Abs(press3)); err != nil {
			fmt.Printf("Ошибка установки давления 3\n")
			return
		}
		ShowMessage(" ", 2)
		fmt.Printf("Давление 3: %.2f кгс/см2 (%v)\n", math.Abs(press3), err)
		gForm.EntryPress3.Entry.SetText(fmt.Sprintf("%.2f", math.Abs(press3)))
	}

	startPressButton = widget.NewButton("Ок", func() {
		entryPress1.Entry.OnSubmitted(entryPress1.Entry.Text)
		gForm.EntryPress2.Entry.OnSubmitted(gForm.EntryPress2.Entry.Text)
		gForm.EntryPress3.Entry.OnSubmitted(gForm.EntryPress3.Entry.Text)
	})

	box3 := container.NewGridWithColumns(
		3,
		widget.NewLabel("Канал 1 (ТМ):"), widget.NewLabel("Канал 2 (ТЦ):"), widget.NewLabel("Канал 3 (ГР):"),
		entryPress1, gForm.EntryPress2, gForm.EntryPress3,
		dummy, dummy, startPressButton,
	)
	boxPress := container.NewVBox(getTitle("Имитация давления (кгс/см²):"), box3)

	// boxAll := container.NewVBox(layout.NewSpacer(), boxSpeed, boxMileage, boxPress, dummy, layout.NewSpacer())
	boxAll := container.NewVBox(customDummy, boxSpeed, boxMileage, boxPress, customDummy) // dummy,
	box := container.NewHBox(dummy, boxAll, dummy)

	return box
}

//---------------------------------------------------------------------------//
// 						ИНТЕРФЕЙС: ФДС сигналы
//---------------------------------------------------------------------------//

// коды РЦ (Сигналы ИФ)
// Вых.БУ: 50В, 10В
func outputSignals() fyne.CanvasObject {
	var err error
	pin := uint(0)
	dummy := widget.NewLabel("")

	code := []string{"Ноль",
		"КЖ 1.6",
		"Ж 1.6",
		"З 1.6",
		"КЖ 1.9",
		"Ж 1.9",
		"З 1.9",
	}
	gForm.Radio = widget.NewRadioGroup(code, func(s string) {
		fds.SetIF(ipk.IFEnable)
		switch s {
		case "Ноль":
			err = fds.SetIF(ipk.IFDisable)
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
	gForm.Radio.SetSelected("Ноль")
	fds.SetIF(ipk.IFDisable) // предустановка
	// radio.Horizontal = true
	boxCode := container.NewVBox(dummy, getTitle("Коды РЦ:      "), gForm.Radio)

	// 10V
	// out10V, _ := fds.UintGetOutput10V()

	checkG := widget.NewCheck("З", func(on bool) {
		pin = 0
		if on {
			err = fds.Set10V(pin, true)
		} else {
			err = fds.Set10V(pin, false)
		}
		fmt.Printf("Двоичные выходы 10В: %d=%v З (%v)\n", pin, on, err)
	})
	fds.Set10V(0, false) // предустановка
	checkG.SetChecked(false)
	// pin10V := uint8(0x01)
	// if (out10V & pin10V) == pin10V {
	// 	checkG.SetChecked(true)
	// }

	checkY := widget.NewCheck("Ж", func(on bool) {
		pin = 1
		if on {
			err = fds.Set10V(pin, true)
		} else {
			err = fds.Set10V(pin, false)
		}
		fmt.Printf("Двоичные выходы 10В: %d=%v Ж(%v)\n", pin, on, err)
	})
	fds.Set10V(1, false)
	checkY.SetChecked(false)

	checkRY := widget.NewCheck("КЖ", func(on bool) {
		pin = 2
		if on {
			err = fds.Set10V(pin, true)
		} else {
			err = fds.Set10V(pin, false)
		}
		fmt.Printf("Двоичные выходы 10В: %d=%v КЖ (%v)\n", pin, on, err)
	})
	fds.Set10V(2, false)
	checkRY.SetChecked(false)

	checkR := widget.NewCheck("К", func(on bool) {
		pin = 3
		if on {
			err = fds.Set10V(pin, true)
		} else {
			err = fds.Set10V(pin, false)
		}
		fmt.Printf("Двоичные выходы 10В: %d=%v К (%v)\n", pin, on, err)
	})
	fds.Set10V(3, false)
	checkR.SetChecked(false)

	checkW := widget.NewCheck("Б", func(on bool) {
		pin = 4
		if on {
			err = fds.Set10V(pin, true)
		} else {
			err = fds.Set10V(pin, false)
		}
		fmt.Printf("Двоичные выходы 10В: %d=%v Б (%v)\n", pin, on, err)
	})
	fds.Set10V(4, false)
	checkW.SetChecked(false)

	checkEPK1 := widget.NewCheck("ЭПК1", func(on bool) {
		pin = 5
		if on {
			err = fds.Set10V(pin, true)
		} else {
			err = fds.Set10V(pin, false)
		}
		fmt.Printf("Двоичные выходы 10В: %d=%v ЭПК1 (%v)\n", pin, on, err)
	})
	fds.Set10V(5, false)
	checkEPK1.SetChecked(false)

	checkTracktion := widget.NewCheck("Тяга", func(on bool) {
		pin = 7
		if on {
			if gBU.Variant == BU4 {
				err = fds.Set50V(4, true)
			} else {
				err = fds.Set10V(pin, true)
			}
		} else {
			if gBU.Variant == BU4 {
				err = fds.Set50V(4, false)
			} else {
				err = fds.Set10V(pin, false)
			}
		}
		fmt.Printf("Двоичные выходы 10В: %d=%v Тяга (%v)\n", pin, on, err)
	})
	fds.Set10V(7, false)
	fds.Set50V(4, false)
	checkTracktion.SetChecked(false)
	gForm.BoxOut10V = container.NewVBox(checkG, checkY, checkRY, checkR, checkW, checkEPK1)

	// 50V
	checkLP := widget.NewCheck("ЛП", func(on bool) {
		pin = uint(0)
		if on {
			err = fds.Set50V(pin, true)
		} else {
			err = fds.Set50V(pin, false)
		}
		fmt.Printf("Двоичные выходы 50В: %d=%v ЛП (%v)\n", pin, on, err)
	})
	fds.Set50V(0, false)
	checkLP.SetChecked(false)

	checkButtonUhod := widget.NewCheck("кн. Уход", func(on bool) {
		pin = 2
		if on {
			err = fds.Set50V(pin, true)
		} else {
			err = fds.Set50V(pin, false)
		}
		fmt.Printf("Двоичные выходы 50В: %d=%v кн. Уход (%v)\n", pin, on, err)
	})
	fds.Set50V(2, false)
	checkButtonUhod.SetChecked(false)

	checkEPK := widget.NewCheck("ЭПК", func(on bool) {
		pin = 4
		if on {
			err = fds.Set50V(pin, true)
		} else {
			err = fds.Set50V(pin, false)
		}
		fmt.Printf("Двоичные выходы 50В: %d=%v ЭПК (%v)\n", pin, on, err)
	})
	fds.Set50V(4, false)
	checkEPK.SetChecked(false)

	checkKeyEPK := widget.NewCheck("Ключ ЭПК ", func(on bool) {
		pin = 8
		if on {
			err = fds.Set50V(pin, true)
		} else {
			err = fds.Set50V(pin, false)
		}
		fmt.Printf("Двоичные выходы 50В: %d=%v Ключ ЭПК (%v)\n", pin, on, err)
	})
	fds.Set50V(8, false)
	checkKeyEPK.SetChecked(false)

	gForm.BoxOut50V = container.NewVBox(checkLP, checkButtonUhod, checkEPK, checkKeyEPK)

	boxOut := container.NewVBox(dummy, getTitle("Сигналы БУ:"), checkTracktion, gForm.BoxOut10V, gForm.BoxOut50V)
	// box := container.NewVBox(layout.NewSpacer(), container.NewHBox(boxOut, boxCode), layout.NewSpacer())
	box := container.NewHBox(boxOut, boxCode)

	return box
}

// Уставки, входы БУС
func inputSignals() fyne.CanvasObject {
	currentBU := OptionsBU(BU3PV)

	relay1 := widget.NewCheck("1", nil)
	relay20 := widget.NewCheck("20", nil)
	gForm.RelayY = widget.NewCheck(fmt.Sprintf("%d", gBU.RelayY), nil)   // ~45 V(ж)
	gForm.RelayRY = widget.NewCheck(fmt.Sprintf("%d", gBU.RelayRY), nil) // ~30 V(кж)
	gForm.RelayU = widget.NewCheck(fmt.Sprintf("%d", gBU.RelayU), nil)   // ~10 V(упр)
	boxRelay := container.NewHBox(relay1, relay20, gForm.RelayY, gForm.RelayRY, gForm.RelayU)

	checkPSS2 := widget.NewCheck("ПСС2", nil)
	checkUhod2 := widget.NewCheck("Уход 2", nil)
	checkPowerEPK := widget.NewCheck("Пит.ЭПК", nil)
	checkPB2 := widget.NewCheck("РБ2", nil)
	checkEMV := widget.NewCheck("ЭМВ", nil)
	gForm.BoxBUS = container.NewHBox(checkPSS2, checkUhod2, checkPowerEPK, checkPB2, checkEMV)

	box := container.NewHBox(boxRelay, gForm.BoxBUS)

	checkBoxEnable := func() {
		relay1.Enable()
		relay20.Enable()
		gForm.RelayY.Enable()
		gForm.RelayRY.Enable()
		gForm.RelayU.Enable()

		checkPSS2.Enable()
		checkUhod2.Enable()
		checkPowerEPK.Enable()
		checkPB2.Enable()
		checkEMV.Enable()
	}

	checkBoxDisable := func() {
		relay1.Disable()
		relay20.Disable()
		gForm.RelayY.Disable()
		gForm.RelayRY.Disable()
		gForm.RelayU.Disable()

		checkPSS2.Disable()
		checkUhod2.Disable()
		checkPowerEPK.Disable()
		checkPB2.Disable()
		checkEMV.Disable()
	}

	go func() {
		for {
			if currentBU != gBU.Variant {
				if gBU.Variant == BU4 {
					checkBoxDisable()
				} else {
					checkBoxEnable()
				}
				currentBU = gBU.Variant
			}

			bin, _ := fas.UintGetBinaryInput()

			if bin&0x100 == 0x100 {
				relay1.SetChecked(!true) // все сигналы в этом блоке инвертированы
			} else {
				relay1.SetChecked(!false)
			}
			if bin&0x200 == 0x200 {
				relay20.SetChecked(!true)
			} else {
				relay20.SetChecked(!false)
			}
			if bin&0x400 == 0x400 {
				gForm.RelayY.SetChecked(!true)
			} else {
				gForm.RelayY.SetChecked(!false)
			}
			if bin&0x800 == 0x800 {
				gForm.RelayRY.SetChecked(!true)
			} else {
				gForm.RelayRY.SetChecked(!false)
			}
			if bin&0x1000 == 0x1000 {
				gForm.RelayU.SetChecked(!true)
			} else {
				gForm.RelayU.SetChecked(!false)
			}
			pss2, _ := fas.GetBinaryInputVal(0) // ПСС2
			if pss2 {
				checkPSS2.SetChecked(!true)
			} else {
				checkPSS2.SetChecked(!false)
			}
			uhod2, _ := fas.GetBinaryInputVal(1) // УХОД
			// (проверить: задать скорость больше 2 км/ч без тяги)
			if uhod2 {
				checkUhod2.SetChecked(!true)
			} else {
				checkUhod2.SetChecked(!false)
			}
			epk, _ := fas.GetBinaryInputVal(2) // Пит. ЭПК
			if epk {
				checkPowerEPK.SetChecked(!true)
			} else {
				checkPowerEPK.SetChecked(!false)
			}
			rb2, _ := fas.GetBinaryInputVal(3) // РБC
			if rb2 {
				checkPB2.SetChecked(!true)
			} else {
				checkPB2.SetChecked(!false)
			}
			emv, _ := fas.GetBinaryInputVal(4) // ЭМВ
			if emv {
				checkEMV.SetChecked(!true)
			} else {
				checkEMV.SetChecked(!false)
			}

			time.Sleep(time.Second)
		}
	}()

	return container.NewVBox(getTitle("Реле БУ:"), box)
}

// ---------------------------------------------------------------------------//
//
//	ИНТЕРФЕЙС: верх
//
// ---------------------------------------------------------------------------//
var buttonUPP *widget.Button
var selectDevice *widget.Select

// var resetCheckTurt bool

func top() fyne.CanvasObject {

	// Режим обслуживания
	gForm.CheckTurt = widget.NewCheck("TURT", func(on bool) {
		if on { // если установили чек, а не сбросили
			if gBU.Variant == BU4 {
				_, msg := setServiceModeBU4()
				ShowMessage(msg, 4)
			} else {
				gBU.Turt(on)
				bTurt = true
			}
		} else { // сбросили чек
			if gBU.Variant != BU4 { // off TURT
				gBU.Turt(on)
				bTurt = false
			}
			fmt.Println("Режим обслуживания сброшен")
		}
	})

	gForm.CheckPower = widget.NewCheck("Питание КПД", func(on bool) {
		gBU.Power(on)
	})
	gForm.CheckPower.SetChecked(true)

	// Смена блока туть
	selectDevice = widget.NewSelect(gDeviceChoice, func(s string) {
		config.DeviceVariant = OptionsBU(selectDevice.SelectedIndex())
		writeFyneAPP(config)
		initDataBU(OptionsBU(selectDevice.SelectedIndex()))
		if bConnectedCAN {
			readUPPfromBU()
		} else {
			ShowMessage("Ошибка получения УПП с блока по CAN.", 3)
		}
		refreshForm()
	})
	selectDevice.SetSelectedIndex(int(gBU.Variant)) // предустановка

	buttonUPP = widget.NewButton("  УПП  ", func() {
		ShowMessage(" ", 2)
		buttonUPP.Disable()
		showFormUPP()
		// buttonUPP.Enable()
	})

	box := container.New(layout.NewHBoxLayout(), selectDevice, gForm.CheckPower, gForm.CheckTurt, layout.NewSpacer(), buttonUPP)

	return box
}

func showFormUPP() {
	var paramEntry = make(map[int]*widget.Entry)
	statusLabel := widget.NewLabel(" ")
	managePower := widget.NewCheck("Управлять питанием", nil)
	if gBU.Variant == BU4 || gBU.Variant == BU3P {
		managePower.Hide()
	}
	managePower.SetChecked(true)

	w := fyne.CurrentApp().NewWindow("Установка условно постоянных признаков " + gBU.Name) // CurrentApp!
	w.Resize(fyne.NewSize(800, 650))
	w.SetFixedSize(true)
	w.CenterOnScreen()

	if err := readUPPfromBU(); err == nil {
		statusLabel.SetText("УПП считаны с блока")
	} else {
		statusLabel.SetText(err.Error())
	}

	var temp []int
	for n := range gUPP {
		temp = append(temp, n)
	}
	sort.Ints(temp)

	b := container.NewVBox()
	for _, number := range temp {
		upp := gUPP[number]

		nameLabel := widget.NewLabel(fmt.Sprintf("%-4d %s", number, upp.Name))
		nameLabel.TextStyle.Monospace = true

		paramEntry[number] = widget.NewEntry()
		paramEntry[number].TextStyle.Monospace = true
		paramEntry[number].SetText(upp.Value)
		paramEntry[number].OnChanged = func(str string) {
			str = strings.ReplaceAll(str, ",", ".")
			paramEntry[upp.Mod].SetText(str) // нельзя number!
			statusLabel.SetText(upp.Hint)
		}

		line := container.NewGridWithColumns(2, nameLabel, paramEntry[number])
		b.Add(line)
	}
	boxScrollUPP := container.NewVScroll(b)                                                             // + крутилку
	boxScrollLayoutUPP := container.New(layout.NewGridWrapLayout(fyne.NewSize(770, 550)), boxScrollUPP) // чтобы не расползались, нужно место для кнопок

	// считать УПП записанные в БУ
	readBUButton := widget.NewButton("УПП БУ", func() {
		err := readUPPfromBU()
		if err != nil {
			statusLabel.SetText("Ошибка получения УПП с блока по шине CAN")
		} else {
			statusLabel.SetText("УПП считаны с блока")
		}

		for number, upp := range gUPP {
			paramEntry[number].SetText(upp.Value)
		}
	})

	// записать то что на форме в БУ
	writeButton := widget.NewButton("Записать", func() {

		// проверить все введенные данные на соответствие границам
		tempupp := make(map[int]DataUPP)
		tempupp = gUPP
		for number, upp := range tempupp {
			upp.Value = paramEntry[number].Text
			if err := upp.checkValueUPP(); err != nil {
				statusLabel.SetText(err.Error())
				return
			}
			tempupp[number] = upp
		}
		d1, _ := strconv.ParseFloat(tempupp[2].Value, 32)
		d2, _ := strconv.ParseFloat(tempupp[3].Value, 32)
		if math.Abs(d1-d2) > 20. {
			statusLabel.SetText("Ошибка: диаметры бандажа колёсной пары 1 и 2 отличаются")
		}

		// записать в БУ
		gUPP = tempupp
		if gBU.Variant == BU4 {
			if !isServiceModeBU4() {
				_, msg := setServiceModeBU4()
				statusLabel.SetText(msg)
				time.Sleep(2 * time.Second) // todo
			}
		} else {
			if managePower.Checked == true { // если управляем питанием
				gBU.SetServiceMode()
			}
		}

		if err := writeUPPtoBU(); err != nil {
			statusLabel.SetText(err.Error())
			return
		}
		statusLabel.SetText("УПП записаны успешно")

		writeParamToTOML()
		refreshDataBU()
		refreshForm()

		if gBU.Variant != BU4 && managePower.Checked == true {
			time.Sleep(2 * time.Second)
			gBU.SetOperateMode()
		}

		if gBU.power {
			gForm.CheckPower.SetChecked(true)
		} else {
			gForm.CheckPower.SetChecked(false)
		}
	})

	if gBU.Variant == BU3P {
		writeButton.Hide()
	}
	if bMotion || bTrip {
		writeButton.Disable()
	}

	readTomlButton := widget.NewButton("Сохранённые УПП", func() {
		tomlupp, err := readParamFromTOML() // никуда не сохраняются, только показать на форме
		if err != nil {
			statusLabel.SetText("Ошибка чтения УПП из файла")
		} else {
			statusLabel.SetText("УПП считаны из файла")
		}

		for number, upp := range tomlupp {
			paramEntry[number].SetText(upp.Value)
		}
	})

	boxButtons := container.NewHBox(readBUButton, readTomlButton, layout.NewSpacer(), managePower, writeButton)
	boxBottom := container.NewVBox(statusLabel, boxButtons)
	boxButtonsLayout := container.New(layout.NewGridWrapLayout(fyne.NewSize(800, 80)), boxBottom) // чтобы не расползались кнопки при растягивании бокса

	box := container.NewVBox(boxScrollLayoutUPP, boxButtonsLayout)

	/*
		! При завкрытие окна кнопка снова будет активна
	*/
	w.SetOnClosed(func() {
		buttonUPP.Enable()
	})

	w.SetContent(box)
	w.Show() // ShowAndRun -- panic!
}

func ErrorDialog(title string, dismiss string, message string) {

	dialog.ShowCustomError(
		title,
		dismiss,
		message,
		func(b bool) { // колбек выполняется после принятия решения, принимает bool  в зависимости от решения пользователя
			if b {
				closer.Close()
			}
		},
		w,
	)
}

// Получаем текущую скорость во время движения или поездки
func currentSpeed() {
	for {
		if bTrip || bMotion {
			s1, s2, _ := sp.GetOutputSpeed()
			currentSpeed1.Set(fmt.Sprintf("%.1f", s1))
			currentSpeed2.Set(fmt.Sprintf("%.1f", s2))
		} else {
			currentSpeed1.Set(fmt.Sprintf("%.1f", 0.0))
			currentSpeed2.Set(fmt.Sprintf("%.1f", 0.0))
		}

		time.Sleep(time.Second)
	}
}

/*
После закрытия прораммы требуется время на закрытие канала CAN и т д
Если запустить программу раньше деинициализации - баг (закроется канал в только открытой программе)
*/
func programAlreadyRunning() {
	var name string
	var bStop = true
	var countProcess = 0
	time.AfterFunc(6*time.Second, func() {
		bStop = false // Время отображения сообщения истекло
	})
	for bStop {
		processes, _ := process.Processes()
		for _, process := range processes {
			name, _ = process.Name()
			if name == "testSrs3.exe" {
				countProcess++
				if countProcess >= 2 {
					time.Sleep(time.Second / 2)
					// fmt.Println("testSrs3 уже запущен")
					countProcess = 0
					break
				}
			}
		}

		// вышли из цикла с именем процесса не testSrs3 значит процесс не запущен
		if name != "testSrs3.exe" {
			return
		}
	}
}
