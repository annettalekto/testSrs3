package main

import (
	"errors"
	"fmt"
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

var bOkCAN bool
var bConnected bool

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

	// Инит
	var b candev.Builder
	err := errors.New("")

	can25, err = b.Speed(ixxatvci3.Bitrate25kbps).Get()
	if err != nil {
		bOkCAN = false
		fmt.Printf("Ошибка инициализации CAN: %v\n", err)
		err = errors.New("Ошибка инициализации CAN")
	} else {
		bOkCAN = true
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
	w.Resize(fyne.NewSize(1024, 780))   // прописать точный размер
	w.SetFixedSize(true)                // не использовать без Resize
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

	err = initIPK()
	if err != nil {
		gForm.Status.Set(err.Error())

	} else {
		gForm.Status.Set("IPK init OK")
		fmt.Println("IPK init OK")
	}

	// вывод ошибок полученных при старте программы
	if !errConfig {
		gForm.Status.Set("Не получены данные из файла конфигурации")
	}

	/*
		! Start
	*/
	switch {

	case !bOkCAN:
		ErrorDialog("Ошибка CAN", "Выход", "Подключите CAN адаптер")

	case err != nil:
		ErrorDialog("Ошибка ИПК", "Выход", "Подключите ИПК")

	case err == nil:
		go threadInitUPP()
		go threadActivity()
		go processScreen()
	}

	// запуск формы
	w.SetContent(box)
	w.ShowAndRun()
}

var canOk = make(chan int)

func threadInitUPP() {

	err := errors.New("")
	var canmsg bool // ipk,
	activityWindow()

	sec := time.NewTicker(1 * time.Second)
	for range sec.C {
		if !canmsg {
			// Получение сообщений CAN
			if !bConnected {
				fmt.Println("Блок БУ не обнаружен")
				setStatus("Проверьте подключение CAN. Включите тумбер ИПК (50В) и переведите БУ в режим поездки")

			} else {
				fmt.Println("CAN OK")
				canmsg = true
				// CAN работает, пробуем получить признаки
				if err = readUPPfromBU(); err == nil {
					setStatus("Соединение с БУ установлено")
				} else {
					setStatus(err.Error())
				}
				refreshForm()
			}
		}
		if canmsg { // ipk &&
			fmt.Println("Init OK. Let's work!")
			canOk <- 1
			break
		}
	}
}

var wasConnected bool

/*
Проверка связи с БУ
*/
func threadActivity() {
	<-canOk
	sec := time.NewTicker(1 * time.Second)
	for range sec.C {
		if bConnected {
			setStatus("УПП получены с блока") // Сообщение будет перекрывать все остальные

		} else {
			fmt.Println("Блок БУ не обнаружен")
			setStatus("Проверьте подключение CAN. Включите тумбер ИПК (50В) и переведите БУ в режим поездки")
		}

		if wasConnected != bConnected {
			wasConnected = bConnected
			activityWindow()
			// refreshForm()
		}

		bConnected = false // true установиться в потоке can
	}
}

var textMessage, textStatus string // переменные хранят значение последнего статуса и сообщения
var bShowMessage = false           // признак нового сообщения
var timer *time.Timer

const showTime = 5

// устанавоиваем текущий статус
func setStatus(message string) {
	textStatus = message
}

/*
Показать сообщение
message текст
ShowTime - сколько мс будет отображаться сообщение на экране.
*/
func ShowMessage(message string) {

	if bShowMessage {
		timer.Stop()
	} else {
		bShowMessage = true
	}

	textMessage = message

	timer = time.AfterFunc(showTime*time.Second, func() {
		bShowMessage = false
		// fmt.Println("Время отображения сообщения истекло")
	})
}

// Поток для вывода подсказок и ошибок
func processScreen() {

	sec := time.NewTicker(200 * time.Millisecond)

	for range sec.C {

		// установить
		if bShowMessage {
			gForm.Status.Set(textMessage)
		} else {
			gForm.Status.Set(textStatus) // выводится повторяющеся событие
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
	if isServiceModeBU4() {
		gForm.CheckTurt.SetChecked(true)
	} else {
		gForm.CheckTurt.SetChecked(false)
	}

	// if gBU.NumberDUP == 1 {
	// 	gForm.EntrySpeed2.Entry.Disable()
	// 	gForm.EntryAccel2.Entry.Disable()
	// } else { // gBU.NumberDUP == 2
	// 	if !bConnected {
	// 		gForm.EntrySpeed2.Entry.Disable()
	// 		gForm.EntryAccel2.Entry.Disable()
	// 	} else {
	// 		gForm.EntrySpeed2.Entry.Enable()
	// 		gForm.EntryAccel2.Entry.Enable()
	// 	}
	// }
	// if gBU.NumberDD == 1 {
	// 	gForm.EntryPress2.Entry.Disable()
	// 	gForm.EntryPress3.Entry.Disable()
	// } else { //gBU.NumberDD == 2
	// 	gForm.EntryPress3.Entry.Disable()
	// 	if !bConnected {
	// 		gForm.EntryPress2.Entry.Disable()
	// 	} else {
	// 		gForm.EntryPress2.Entry.Enable()
	// 	}
	// }

	gForm.BoxBUS.Hide()
	gForm.BoxOut50V.Hide()
	gForm.BoxOut10V.Hide()
	gForm.Radio.Hide()

	if major, minor, patch, number, err := canGetVersionBU4(); err == nil {
		gBU.VersionBU4 = fmt.Sprintf("Версия %d.%d.%d (в лоции №%d)", major, minor, patch, number)
	}
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

		// if !bConnected {
		// 	gForm.EntrySpeed2.Entry.Disable()
		// 	gForm.EntryAccel2.Entry.Disable()
		// 	gForm.EntryPress2.Entry.Disable()
		// 	gForm.EntryPress3.Entry.Disable()
		// } else {
		// 	gForm.EntrySpeed2.Entry.Enable()
		// 	gForm.EntryAccel2.Entry.Enable()
		// 	gForm.EntryPress2.Entry.Enable()
		// 	gForm.EntryPress3.Entry.Enable()
		// }

		gForm.BoxOut10V.Show()
		gForm.Radio.Show()
	}

	switch gBU.Variant {
	case BU3P, BU3PA:
		gForm.BoxBUS.Hide()
		gForm.BoxOut50V.Hide()
	case BU3PV:
		gForm.BoxBUS.Show()
		gForm.BoxOut50V.Show()
	case BU4:
		changeFormBU4()
	}

	return

}

// Без соединения с БУ все поля ввода не активны
func activityWindow() {

	if !bConnected {

		startButton.Disable()
		buttonMileage.Disable()
		startPressButton.Disable()
		// gForm.CheckPower.Disable()
		buttonUPP.Disable()

		entryPress1.Disable()
		entryMileage.Disable()
		entrySpeed1.Disable()
		entryAccel1.Disable()
		radioDirection.Disable()
		gForm.CheckTurt.Disable()

		gForm.EntrySpeed2.Entry.Disable()
		gForm.EntryAccel2.Entry.Disable()
		gForm.EntryPress2.Entry.Disable()
		gForm.EntryPress3.Entry.Disable()

		return

	} else {

		startButton.Enable()
		buttonMileage.Enable()
		startPressButton.Enable()
		// gForm.CheckPower.Enable()
		buttonUPP.Enable()

		entryPress1.Enable()
		entryMileage.Enable()
		entrySpeed1.Enable()
		entryAccel1.Enable()
		radioDirection.Enable()

		gForm.CheckTurt.Enable()
		gForm.CheckTurt.SetChecked(false)

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
		}

		return
	}
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
			ShowMessage(fmt.Sprintf("H%s: %s", sCodeError, sErrorDescription))
			list.Unselect(id) // сбросить выделение пункта, чтобы сново можно было по нему тыкать и получать подсказку
		} else {
			ShowMessage("")
		}
	}

	// обновление данных
	go func() {
		for {
			data = nil
			mapDataCAN := getDataCAN()

			t := byteToTimeBU(mapDataCAN[idTimeBU])
			data = append(data, fmt.Sprintf("Время БУ:  %s", t.Format("02.01.2006 15:04")))

			if gBU.Variant == BU4 {
				if gBU.VersionBU4 != "" {
					data = append(data, gBU.VersionBU4)
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
				enterSpeed1, _ := strconv.Atoi(entrySpeed1.Entry.Text)
				enterSpeed2, _ := strconv.Atoi(gForm.EntrySpeed2.Entry.Text)
				if enterSpeed1 > 400 || enterSpeed2 > 400 { // БУ присылает неверные данные, если скорость более 400 км/ч, отслеживаем по введенным данным
					data = append(data, fmt.Sprintf("%-22s Ошибка", "Скорость 1 каб.(км/ч):"))
				} else {
					data = append(data, fmt.Sprintf("%-22s %.1f", "Скорость 1 каб.(км/ч):", byteToSpeed(bytes)))
				}
			} else {
				data = append(data, fmt.Sprintf("%-22s —", "Скорость 1 каб.(км/ч):"))
			}
			if bytes, ok := mapDataCAN[idSpeed2]; ok {
				enterSpeed1, _ := strconv.Atoi(entrySpeed1.Entry.Text)
				enterSpeed2, _ := strconv.Atoi(gForm.EntrySpeed2.Entry.Text)
				if enterSpeed1 > 400 || enterSpeed2 > 400 {
					data = append(data, fmt.Sprintf("%-22s Ошибка", "Скорость 2 каб.(км/ч):"))
				} else {
					data = append(data, fmt.Sprintf("%-22s %.1f", "Скорость 2 каб.(км/ч):", byteToSpeed(bytes)))
				}
			} else {
				data = append(data, fmt.Sprintf("%-22s —", "Скорость 2 каб.(км/ч):"))
			}

			if bytes, ok := mapDataCAN[idPressure]; ok {
				tm, tc, gr := byteToPressure(bytes)
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
			time.Sleep(2 * time.Second)
		}
	}()

	box := container.NewBorder(getTitle("Данные CAN:"), nil, nil, nil, list)

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

	go func() {
		stop := false
		ch, _ := can25.GetMsgChannelCopy()

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

					if msg.ID == idTimeBU {
						bConnected = true
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

var startButton *widget.Button
var buttonMileage *widget.Button
var startPressButton *widget.Button

// Скорость, дистанция, давление
func speed() fyne.CanvasObject {
	var err error

	// ------------------------- box 1 ----------------------------

	separately := binding.NewBool() // cовместное или раздельное управление
	direction := uint8(ipk.MotionOnward)
	speed1, speed2, accel1, accel2 := float64(0), float64(0), float64(0), float64(0)
	speedLimit := 1000
	dummy := widget.NewLabel("")

	// обработка скорости
	entrySpeed1 = newSpecialEntry("0")
	gForm.EntrySpeed2 = newSpecialEntry("0")

	entrySpeed1.Entry.OnChanged = func(str string) {
		if str == "" {
			return
		}
		str = strings.ReplaceAll(str, ",", ".")
		if speed1, err = strconv.ParseFloat(str, 64); err != nil {
			fmt.Printf("Ошибка перевода строки в число (скорость 1)\n")
			ShowMessage("Ошибка в поле ввода «Скорость 1»")
			return
		}
		if sep, _ := separately.Get(); !sep {
			speed2 = speed1
			gForm.EntrySpeed2.SetText(str)
		}
		if speed1 > float64(speedLimit) {
			ShowMessage(fmt.Sprintf("Скорость 1: максимум %d км/ч", speedLimit))
		}
		if speed2 > float64(speedLimit) {
			ShowMessage(fmt.Sprintf("Скорость 2: максимум %d км/ч", speedLimit))
		}
	}
	entrySpeed1.Entry.OnSubmitted = func(str string) {
		selectAll()
		if speed1 > float64(speedLimit) || speed2 > float64(speedLimit) {
			ShowMessage("Ошибка установки скорости")
			return
		}
		if err = sp.SetSpeed(speed1, speed2); err != nil {
			fmt.Printf("Ошибка установки скорости")
			ShowMessage("Ошибка установки скорости")
			return
		}
		ShowMessage(" ")
		if strings.Contains(str, ".") {
			entrySpeed1.Entry.SetText(fmt.Sprintf("%.1f", speed1))
			gForm.EntrySpeed2.Entry.SetText(fmt.Sprintf("%.1f", speed2))
		} else {
			entrySpeed1.Entry.SetText(fmt.Sprintf("%.0f", speed1))
			gForm.EntrySpeed2.Entry.SetText(fmt.Sprintf("%.0f", speed2))
		}
		fmt.Printf("Скорость: %.1f %.1f км/ч (%v)\n", speed1, speed2, err)
	}

	gForm.EntrySpeed2.Entry.OnChanged = func(str string) {
		if str == "" {
			return
		}
		str = strings.ReplaceAll(str, ",", ".")
		if speed2, err = strconv.ParseFloat(str, 64); err != nil {
			fmt.Printf("Ошибка перевода строки в число (скорость 2)\n")
			ShowMessage("Ошибка в поле ввода «Скорость 2»")
			return
		}
		if sep, _ := separately.Get(); !sep {
			speed1 = speed2
			entrySpeed1.Entry.SetText(str)
		}
		if speed1 > float64(speedLimit) {
			ShowMessage(fmt.Sprintf("Скорость 1: максимум %d км/ч", speedLimit))
		}
		if speed2 > float64(speedLimit) {
			ShowMessage(fmt.Sprintf("Скорость 2: максимум %d км/ч", speedLimit))
		}
	}
	gForm.EntrySpeed2.Entry.OnSubmitted = func(str string) {
		selectAll()
		if speed1 > float64(speedLimit) || speed2 > float64(speedLimit) {
			ShowMessage("Ошибка установки скорости")
		}
		if err = sp.SetSpeed(speed1, speed2); err != nil {
			fmt.Printf("Ошибка установки скорости")
			ShowMessage("Ошибка установки скорости")
			return
		}
		ShowMessage(" ")
		if strings.Contains(str, ".") {
			entrySpeed1.Entry.SetText(fmt.Sprintf("%.1f", speed1))
			gForm.EntrySpeed2.Entry.SetText(fmt.Sprintf("%.1f", speed2))
		} else {
			entrySpeed1.Entry.SetText(fmt.Sprintf("%.0f", speed1))
			gForm.EntrySpeed2.Entry.SetText(fmt.Sprintf("%.0f", speed2))
		}
		fmt.Printf("Скорость: %.1f %.1f  (%v)\n", speed1, speed2, err)
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
			ShowMessage("Ошибка в поле ввода «Ускорение 1»")
			return
		}
		if sep, _ := separately.Get(); !sep {
			accel2 = accel1
			gForm.EntryAccel2.Entry.SetText(str)
		}
		if accel1 > accelLimit {
			ShowMessage(fmt.Sprintf("Ускорение 1: максимум %.0f км/ч", accelLimit))
		}
		if accel2 > accelLimit {
			ShowMessage(fmt.Sprintf("Ускорение 2: максимум %.0f км/ч", accelLimit))
		}
	}
	entryAccel1.Entry.OnSubmitted = func(str string) {
		selectAll()
		if accel1 > accelLimit || accel2 > accelLimit {
			ShowMessage("Ошибка установки ускорения")
			return
		}
		if err = sp.SetAcceleration(accel1*100, accel2*100); err != nil {
			fmt.Printf("Ошибка установки ускорения\n")
			ShowMessage("Ошибка установки ускорения")
			return
		}
		ShowMessage(" ")
		entryAccel1.Entry.SetText(fmt.Sprintf("%.2f", accel1))
		gForm.EntryAccel2.Entry.SetText(fmt.Sprintf("%.2f", accel2))
		fmt.Printf("Ускорение: %.1f %.1f м/с2 (%v)\n", accel1, accel2, err)
	}

	gForm.EntryAccel2.Entry.OnChanged = func(str string) {
		if str == "" {
			return
		}
		str = strings.ReplaceAll(str, ",", ".")
		if accel2, err = strconv.ParseFloat(str, 64); err != nil {
			fmt.Printf("Ошибка перевода строки в число (ускорение 2)\n")
			ShowMessage("Ошибка в поле ввода «Ускорение 2»")
			return
		}
		if sep, _ := separately.Get(); !sep {
			accel1 = accel2
			entryAccel1.Entry.SetText(str)
		}
		if accel1 > accelLimit {
			ShowMessage(fmt.Sprintf("Ускорение 1: максимум %.0f км/ч", accelLimit))
		}
		if accel2 > accelLimit {
			ShowMessage(fmt.Sprintf("Ускорение 2: максимум %.0f км/ч", accelLimit))
		}
	}
	gForm.EntryAccel2.Entry.OnSubmitted = func(str string) {
		selectAll()
		if accel1 > accelLimit || accel2 > accelLimit {
			ShowMessage("Ошибка установки ускорения")
			return
		}
		if err = sp.SetAcceleration(accel1*100, accel2*100); err != nil {
			fmt.Printf("Ошибка установки ускорения\n")
			ShowMessage("Ошибка установки ускорения")
			return
		}
		ShowMessage(" ")
		entryAccel1.Entry.SetText(fmt.Sprintf("%.2f", accel1))
		gForm.EntryAccel2.Entry.SetText(fmt.Sprintf("%.2f", accel2))
		fmt.Printf("Ускорение: %.1f %.1f м/с2 (%v)\n", accel1, accel2, err)
	}

	// тестировщик очень хочет тут кнопку
	startButton = widget.NewButton("Старт", func() {
		entrySpeed1.Entry.OnSubmitted(entrySpeed1.Entry.Text)
		gForm.EntrySpeed2.Entry.OnSubmitted(gForm.EntrySpeed2.Entry.Text)
		entryAccel1.Entry.OnSubmitted(entryAccel1.Entry.Text)
		gForm.EntryAccel2.Entry.OnSubmitted(gForm.EntryAccel2.Entry.Text)
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
			ShowMessage("Ошибка установки направления движения")
			return
		}
		fmt.Printf("Направление: %s\n", s)
		ShowMessage(" ")
	})
	radioDirection.Horizontal = true
	radioDirection.SetSelected("Вперёд")

	separatlyCheck := widget.NewCheckWithData("Раздельное управление", separately)

	// labelParameters := widget.NewLabel("")
	// gForm.Parameters = binding.NewString()
	// labelParameters.Bind(gForm.Parameters)
	// gForm.Parameters.Set(fmt.Sprintf("Число зубьев %d, диаметр бандажа %d мм", gBU.NumberTeeth, gBU.BandageDiameter))

	box1 := container.NewGridWithColumns(
		3,
		dummy, widget.NewLabel("Канал 1"), widget.NewLabel("Канал 2"),
		widget.NewLabel("Скорость (км/ч):"), entrySpeed1, gForm.EntrySpeed2,
		widget.NewLabel("Ускорение (м/с²):"), entryAccel1, gForm.EntryAccel2,
		widget.NewLabel(""), widget.NewLabel(""), startButton,
	)

	boxSpeed := container.NewVBox(getTitle("Имитация движения:"), box1, separatlyCheck, radioDirection /*, labelParameters*/)

	// ------------------------- box 2 ----------------------------

	distanceCheck := false
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
			ShowMessage("Ошибка в поле ввода «Дистанция»: введите целое число")
			return
		}
		d, err := strconv.Atoi(str)
		if err != nil {
			setDistance = 0
			fmt.Printf("Ошибка перевода строки в число (путь)\n")
			ShowMessage("Ошибка в поле ввода «Дистанция»")
			return
		}
		setDistance = uint32(d)

		if setDistance > distanceLimit {
			ShowMessage(fmt.Sprintf("Дистанция: максимум %d м", distanceLimit))
		}
	}

	startMileage := func() bool {
		if setDistance > distanceLimit {
			ShowMessage("Ошибка установки пути")
			return false
		}
		if err = sp.SetLimitWay(setDistance); err != nil {
			fmt.Printf("Ошибка установки пути\n")
			ShowMessage("Ошибка установки пути")
			return false
		}
		time.Sleep(1 * time.Second) // не успевает сбросится счетчик
		if startDistance, _, err = sp.GetWay(); err != nil {
			fmt.Printf("Ошибка: не получено значение пути с ИПК\n")
			ShowMessage("Ошибка: не получено значение пути с ИПК")
			return false
		}
		ShowMessage(" ")
		fmt.Printf("Путь: %d м (%v)\n", setDistance, err)
		distanceCheck = true
		entryMileage.Entry.SetText(fmt.Sprintf("%d", setDistance))
		// скорость должны установить сами в поле ввода скорости
		return true
	}

	stopMileage := func() {
		setDistance = 0
		sp.SetSpeed(0, 0)
		sp.SetAcceleration(0, 0)
		time.Sleep(1 * time.Second)
		startMileage()
		currentDistance.Set("0")
	}

	// запуск по нажатию кнопки

	buttonMileage = widget.NewButton("Ок", func() {
		if !distanceCheck {
			if startMileage() {
				buttonMileage.SetText("Стоп")
			}
		} else {
			stopMileage()
		}
	})
	labelMileage := widget.NewLabel("0")
	labelMileage.Bind(currentDistance)

	// запуск по нажатию Enter
	entryMileage.Entry.OnSubmitted = func(str string) {
		if !distanceCheck {
			if startMileage() {
				buttonMileage.SetText("Стоп")
			}
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
			if distanceCheck {
				m, _, err := sp.GetWay()
				if err != nil {
					fmt.Printf("Не получено значение пути с ИПК\n")
					ShowMessage("Ошибка: не получено значение пути с ИПК")
					break
				} else {
					ShowMessage(" ")
				}
				fmt.Println(m)
				m -= startDistance
				currentDistance.Set(fmt.Sprintf("%d", m))

				if m >= setDistance {
					distanceCheck = false
					buttonMileage.SetText("Ок")
					fmt.Println("Дистанция пройдена")
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
			ShowMessage("Ошибка в поле ввода «Давление 1»")
			return
		}
		if press1 > limit1 {
			ShowMessage(fmt.Sprintf("Давление 1: максимум %.0f кгс/см2", limit1))
		}
		if press1 < 0 {
			ShowMessage(fmt.Sprintf("Давление должно быть положительным"))
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
			ShowMessage("Ошибка установки давления 1")
			return
		}
		ShowMessage(" ")
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
			ShowMessage("Ошибка в поле ввода «Давление 2»")
			return
		}
		limit2 = gBU.PressureLimit
		if press2 > limit2 {
			ShowMessage(fmt.Sprintf("Давление 2: максимум %.0f кгс/см2", limit2))
		}
		if press2 < 0 {
			ShowMessage(fmt.Sprintf("Давление должно быть положительным"))
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
			ShowMessage("Ошибка установки давления 2")
			return
		}
		ShowMessage(" ")
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
			ShowMessage("Ошибка в поле ввода «Давление 3»")
			return
		}
		if press3 > limit3 {
			ShowMessage(fmt.Sprintf("Давление 3: максимум %.0f кгс/см2", limit3))
		}
		if press3 < 0 {
			ShowMessage(fmt.Sprintf("Давление должно быть положительным"))
		}
	}
	gForm.EntryPress3.Entry.OnSubmitted = func(str string) {
		selectAll()
		if err = channel3.Set(math.Abs(press3)); err != nil {
			fmt.Printf("Ошибка установки давления 3\n")
			return
		}
		ShowMessage(" ")
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
	boxAll := container.NewVBox(dummy, boxSpeed, boxMileage, boxPress, dummy)
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

func top() fyne.CanvasObject {

	// Режим обслуживания
	gForm.CheckTurt = widget.NewCheck("TURT", func(on bool) {
		if gBU.Variant == BU4 {
			ok, msg := setServiceModeBU4()
			ShowMessage(msg)
			if !ok {
				gForm.CheckTurt.SetChecked(false)
			} else {
				gForm.CheckTurt.Disable() // выход из режима - перезагрузка
			}

		} else {
			gBU.Turt(on)
		}
	})
	// Смена блока туть
	var selectDevice *widget.Select
	selectDevice = widget.NewSelect(gDeviceChoice, func(s string) {
		config.DeviceVariant = OptionsBU(selectDevice.SelectedIndex())
		writeFyneAPP(config)
		initDataBU(OptionsBU(selectDevice.SelectedIndex()))
		readUPPfromBU()
		refreshForm()
	})
	selectDevice.SetSelectedIndex(int(gBU.Variant)) // предустановка

	gForm.CheckPower = widget.NewCheck("Питание КПД", func(on bool) {
		gBU.Power(on)
		ShowMessage(" ")
		if on && gBU.Variant == BU4 {
			// для БУ-4 выход из режима обслуживания - перезагрузка
			if bConnected {
				gForm.CheckTurt.Enable()
				gForm.CheckTurt.SetChecked(false)
			}
		}
	})
	gForm.CheckPower.SetChecked(true)

	buttonUPP = widget.NewButton("  УПП  ", func() {
		ShowMessage(" ")
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
	if gBU.Variant == BU4 {
		managePower.Hide()
	}
	managePower.SetChecked(true)

	w := fyne.CurrentApp().NewWindow("Установка условно постоянных признаков " + gBU.Name) // CurrentApp!
	w.Resize(fyne.NewSize(800, 600))
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
			}
		} else {
			if managePower.Checked == true {
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
