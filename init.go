package main

import (
	"errors"
	"time"

	"github.com/amdf/ipk"
	"github.com/amdf/ixxatvci3/candev"
)

var can25 candev.Device
var ipkBox ipk.IPK

var sp ipk.Speed
var fas *ipk.AnalogDevice
var fds *ipk.BinaryDevice
var fcs *ipk.FreqDevice

var channel1 ipk.PressureOutput // sensorTM Переменная для задания давления ТM в кгс/см² (канал 1)
var channel2 ipk.PressureOutput // sensorTC Переменная для задания давления ТЦ в кгс/см² (канал 2)
var channel3 ipk.PressureOutput // sensorGR Переменная для задания давления GR в кгс/см²

var gBU DescriptionBU
var gDeviceChoice = []string{"БУ-3П", "БУ-3ПА", "БУ-3ПВ", "БУ-4"} // +kpd +CH? todo

// OptionsBU варианты подключаемых БУ:
const (
	BU3P = iota
	BU3PA
	BU3PV
	BU4
)

// OptionsBU варианты подключаемых БУ
type OptionsBU int

// DescriptionBU основные значения БУ
type DescriptionBU struct {
	Name             string
	Variant          OptionsBU
	Power            bool
	BandageDiameter1 uint32
	BandageDiameter2 uint32
	PressureLimit    float64
	NumberTeeth      uint32
	ScaleLimit       uint32
	RelayY           int
	RelayRY          int
	RelayU           int
}

func initData() {
	gBU.Variant = BU3PV
	gBU.Name = gDeviceChoice[BU3PV]

	// readUPPfromTOML()     // читаем имена признаков БУ, подсказки, предустановленные значения
	// err = readUPPfromBU() // читаем значения в блоке, с ними будет инициализироваться ИПК

	return
}

func initIPK() (err error) {

	ipkBox.AnalogDev = new(ipk.AnalogDevice)
	ipkBox.BinDev = new(ipk.BinaryDevice)
	ipkBox.FreqDev = new(ipk.FreqDevice)

	if !ipkBox.AnalogDev.Open() { //открываем ФАС-3
		err = errors.New("ошибка инициализации ФАС")
		return
	}
	if !ipkBox.BinDev.Open() { //открываем ФДС-3
		err = errors.New("ошибка инициализации ФДС")
		return
	}
	if !ipkBox.FreqDev.Open() { //открываем ФЧС-3
		err = errors.New("ошибка инициализации ФЧС")
		return
	}
	fas = ipkBox.AnalogDev
	fds = ipkBox.BinDev
	fcs = ipkBox.FreqDev

	if err = InitFreqIpkChannel(); err != nil {
		err = errors.New("InitFreqIpkChannel(): " + err.Error())
		return
	}

	// открываем ЦАП 5
	channelN5 := new(ipk.DAC)
	if channelN5.Init(fas, ipk.DAC5); err != nil {
		err = errors.New("ошибка инициализации ЦАП 5: " + err.Error())
		return
	}

	// открываем ЦАП 6
	channelN6 := new(ipk.DAC)
	if channelN6.Init(fas, ipk.DAC6); err != nil {
		err = errors.New("ошибка инициализации ЦАП 6: " + err.Error())
		return
	}

	// открываем ЦАП 7
	channelN7 := new(ipk.DAC)
	channelN7.Init(fas, ipk.DAC7)

	if channel1.Init(channelN5, ipk.DACAtmosphere, 10); err != nil { // максимальное давление 10 кгс/см² (= 10 технических атмосфер) соответствует 20 мА
		err = errors.New("ошибка инициализации ЦАП 5: " + err.Error())
		return
	}

	if channel2.Init(channelN6, ipk.DACAtmosphere, gBU.PressureLimit); err != nil {
		err = errors.New("ошибка инициализации ЦАП 6: " + err.Error())
		return
	}

	if channel3.Init(channelN7, ipk.DACAtmosphere, 10); err != nil { // макс?
		err = errors.New("ошибка инициализации ЦАП 7: " + err.Error())
		return
	}

	return
}

// InitFreqIpkChannel init
func InitFreqIpkChannel() (err error) {

	if err = sp.Init(fcs, gBU.NumberTeeth, gBU.BandageDiameter1); err == nil {

		go func() { // начинаем в фоне обновлять данные по скорости
			for {
				fcs.UpdateFreqDataUSB()
				time.Sleep(time.Second / 4)
				// fmt.Printf("4SP ")
			}
		}()
	}
	return
}

func powerBU(on bool) {
	fds.Set50V(7, on)
}

func turt(on bool) {
	fds.SetTURT(on)
}

func getNameTOML() (s string) {

	switch gBU.Variant {
	case BU3P:
		s = ".\\toml\\bu3p.toml"
	case BU3PA:
		s = ".\\toml\\bu3pa.toml"
	case BU3PV:
		s = ".\\toml\\bu3pv.toml"
	case BU4:
		s = ".\\toml\\bu4.toml"
	}
	return
}
