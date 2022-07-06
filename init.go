package main

import "github.com/amdf/ixxatvci3/candev"

// import (
// 	"errors"
// 	"ipk"
// 	"strconv"
// 	"time"

// 	"github.com/amdf/ixxatvci3/candev"
// )

var can25 candev.Device

// var ipkBox ipk.IPK
// var sp ipk.Speed
// var fas *ipk.AnalogDevice
// var fds *ipk.BinaryDevice
// var fcs *ipk.FreqDevice

// var channel1 ipk.PressureOutput // sensorTM Переменная для задания давления ТM в кгс/см² (канал 1)
// var channel2 ipk.PressureOutput // sensorTC Переменная для задания давления ТЦ в кгс/см² (канал 2)
// var channel3 ipk.PressureOutput // sensorGR Переменная для задания давления GR в кгс/см²

// func initIPK() (err error) {

// 	ipkBox.AnalogDev = new(ipk.AnalogDevice)
// 	ipkBox.BinDev = new(ipk.BinaryDevice)
// 	ipkBox.FreqDev = new(ipk.FreqDevice)

// 	if !ipkBox.AnalogDev.Open() { //открываем ФАС-3
// 		err = errors.New("ошибка инициализации ФАС")
// 		return
// 	}
// 	if !ipkBox.BinDev.Open() { //открываем ФДС-3
// 		err = errors.New("ошибка инициализации ФДС")
// 		return
// 	}
// 	if !ipkBox.FreqDev.Open() { //открываем ФЧС-3
// 		err = errors.New("ошибка инициализации ФЧС")
// 		return
// 	}
// 	fas = ipkBox.AnalogDev
// 	fds = ipkBox.BinDev
// 	fcs = ipkBox.FreqDev

// 	if err = InitFreqIpkChannel(); err != nil {
// 		err = errors.New("InitFreqIpkChannel(): " + err.Error())
// 		return
// 	}

// 	// открываем ЦАП 5
// 	channelN5 := new(ipk.DAC)
// 	if channelN5.Init(fas, ipk.DAC5); err != nil {
// 		err = errors.New("ошибка инициализации ЦАП 5: " + err.Error())
// 		return
// 	}

// 	// открываем ЦАП 6
// 	channelN6 := new(ipk.DAC)
// 	if channelN6.Init(fas, ipk.DAC6); err != nil {
// 		err = errors.New("ошибка инициализации ЦАП 6: " + err.Error())
// 		return
// 	}

// 	// открываем ЦАП 7
// 	channelN7 := new(ipk.DAC)
// 	channelN7.Init(fas, ipk.DAC7)

// 	if channel1.Init(channelN5, ipk.DACAtmosphere, 10); err != nil { // максимальное давление 10 кгс/см² (= 10 технических атмосфер) соответствует 20 мА
// 		err = errors.New("ошибка инициализации ЦАП 5: " + err.Error())
// 		return
// 	}

// 	fPressureLimit, _ := strconv.ParseFloat(valuePressureLimit, 64)
// 	if channel2.Init(channelN6, ipk.DACAtmosphere, fPressureLimit); err != nil {
// 		err = errors.New("ошибка инициализации ЦАП 6: " + err.Error())
// 		return
// 	}

// 	if channel3.Init(channelN7, ipk.DACAtmosphere, 10); err != nil { // макс?
// 		err = errors.New("ошибка инициализации ЦАП 7: " + err.Error())
// 		return
// 	}

// 	return
// }

// // InitFreqIpkChannel init
// func InitFreqIpkChannel() (err error) {
// 	iBandageDiameter1, err := strconv.ParseInt(valueBandageDiameter1, 10, 32)
// 	if err != nil {
// 		return
// 	}
// 	iNumberTeeth, err := strconv.ParseInt(valueNumberTeeth, 10, 32)
// 	if err != nil {
// 		return
// 	}
// 	if err = sp.Init(fcs, uint32(iNumberTeeth), uint32(iBandageDiameter1)); err == nil {

// 		go func() { // начинаем в фоне обновлять данные по скорости
// 			for {
// 				fcs.UpdateFreqDataUSB()
// 				time.Sleep(time.Second / 4)
// 				// fmt.Printf("4SP ")
// 			}
// 		}()
// 	}
// 	return
// }

//-------------------------------------------------------------------------------//
// УПП

// Наименования УПП
const (
	// Properties
	nameBandageDiameter1        = "2 Диаметр бандажа первой колесной пары"
	nameBandageDiameter2        = "3 Диаметр бандажа второй колесной пары"
	namePresenceMPME            = "4 Наличие МПМЭ"
	nameLocomotiveType          = "5 Тип локомотива или электросекции"
	nameLocomotiveNumber        = "6 Номер локомотива или электросекции"
	nameNumberTeeth             = "7 Число зубьев модулятора ДУП"
	nameScaleLimit              = "8 Верхний предел шкалы"
	nameTrackDiscreteness       = "9 Дискретность регистрации пути"      // для БР-2М/1
	nameSpeedDiscreteness       = "10 Дискретность регистрации скорости" // для БР-2М/1
	namePresenceBR2M            = "11 Признак наличия БР-2М/1"
	namePresenceSN              = "11 Признак наличия СН/БЛОК"
	namePressureLimit           = "12 Верхний предел измерения давления в ТЦ" // 3П: в главном резервуаре (по второму каналу) нужен отдельный признак?
	namePresenceSpeedControl    = "13 Наличие контроля скорости"              // для 3П нужен отдельный!
	namePresenceBlockControl    = "13 Наличие блока контроля"                 // для 3ПА, 3ПВ
	nameSetSpeedY               = "14 Уставка скорости V(ж)"
	nameSetSpeedRY              = "15 Уставка скорости V(кж)"
	nameSetSpeedU               = "16 Уставка скорости V(упр)"
	nameNumberOfCabin           = "17 Признак одной или двух кабин или МВПС"
	nameVariantALS              = "18 Код варианта системы АЛС"
	namePresenceBUS             = "19 Признак наличия БУС"
	nameTrackOfPulseGS          = "20 Путь на один импульс гребнесмазывателя"
	namePresenceKVARTA          = "21 Наличие комплекса КВАРТА"
	nameDensityDiscreteness     = "22 Дискретность регистрации топлива"
	nameNumberOfAddParameters   = "25 Число дополнительных параметров"
	nameDigitsInPersonnelNumber = "26 Число разрядов в табельном номере"
)

var valueBandageDiameter1 string
var valueBandageDiameter2 string
var valuePresenceMPME string
var valuePresenceSN string
var valueLocomotiveType string
var valueLocomotiveNumber string
var valueNumberTeeth string
var valueTrackDiscreteness string
var valueSpeedDiscreteness string
var valuePresenceBR2M string
var valuePressureLimit string
var valuePresenceSpeedControl string
var valuePresenceBlockControl string
var valueSetSpeedY string
var valueSetSpeedRY string
var valueSetSpeedU string
var valueNumberOfCabin string
var valueVariantALS string
var valuePresenceBUS string
var valueTrackOfPulseGS string
var valuePresenceKVARTA string
var valueDensityDiscreteness string
var valueNumberOfAddParameters string
var valueDigitsInPersonnelNumber string
var valuePressureUint string // 0 - кгс/см2, 1 - кПа
