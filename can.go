package main

import (
	"encoding/binary"
	"time"
)

// import (
// 	"encoding/binary"
// 	"errors"
// 	"fmt"
// 	"time"

// 	"github.com/amdf/ixxatvci3/candev"
// )

// /*
// 0x50 Состояние сигналов АЛС; 8 байт
// 6-й байт: биты 0-3 коды огней светофора многозначной АЛС,
// Коды:  0 – белый, 1 - красный, 2 – КЖ, 3 – желтый, 4 – зеленый, 5-7 – нет огня
// */

// // получить код АЛС
// func canGetALS() (als int, err error) {
// 	var msg candev.Message

// 	if msg, err = can25.GetMsgByID(0x50, 5*time.Second); err == nil {
// 		als = int(msg.Data[5] & 0xF)
// 		fmt.Printf("CAN Cигнал АЛС: %X, %v\n", als, err)
// 	} else {
// 		err = errors.New("canGetALS(): " + err.Error())
// 		// lg.Error(fmt.Sprintf("%v\r\n", err))
// 	}
// 	return
// }

// /*
// Значения двоичных сигналов 5F8Н Длина: 4 байта.
// 2-й байт (не с нуля), бит 4 тяга (с нуля)
// должна передаваться по каналу CAN не реже 1 раза в секунду
// */

// // получть значение сигнала Тяги
// func canGetSignalTraction() (t bool, err error) {
// 	var msg candev.Message

// 	if msg, err = can25.GetMsgByID(0x5F8, 3*time.Second); err == nil {
// 		if msg.Data[1]&0x10 == 0x10 {
// 			t = true
// 		}
// 	} else {
// 		err = errors.New("canGetSignalTraction(): " + err.Error())
// 		// lg.Error(fmt.Sprintf("%v\r\n", err))
// 	}
// 	// fmt.Printf("Cигнала ТЯГА: %v, %v\n", t, err)

// 	return
// }

// /* 5C5H Состояние периода кодирования и кода рельсовой цепи. Длина: 3 байта.
// 1-й байт: период кодирования (16, 19, 60);
// 2-й байт: время после последнего полного периода (время сбоя) в секундах;
// 3-й байт: код рельсовой цепи (0 – сбой кода; 1 – код зеленого огня; 2 – код желтого с красным огня; 3 – код желтого огня).
// */

// // получить состояние периода кодирования и кода рельсовой цепи
// func canGetCodeIF() (period, t, code int, err error) {
// 	var msg candev.Message

// 	if msg, err = can25.GetMsgByID(0x5C5, 3*time.Second); err == nil {
// 		period = int(msg.Data[0]) // 16 19 60
// 		t = int(msg.Data[1])
// 		code = int(msg.Data[2]) // 0 1 2 3
// 	} else {
// 		err = errors.New("canGetCodeIF(): " + err.Error())
// 		// lg.Error(fmt.Sprintf("%v\r\n", err))
// 	}
// 	fmt.Printf("Период кодирования: %d, код рельсовой цепи: %d, %v\n", period, code, err)

// 	return
// }

/* В MiniMon байты переставлены
Значение скорости. Длина: 2 байта.
1-й байт:
биты 0-7           младшие биты скорости;
2-й байт:
биты 0-3           старшие биты скорости,
биты 4-6           не используются,
бит 7   единица измерения, в которой  представлено
                значение скорости: 0 – км/ч, 1 – 0,1 км/ч.
*/

// получить скорость в км/ч
func byteToSpeed(data [8]byte) (s float64) {
	h := uint16(data[1]&0xF) << 8
	val := h | uint16(data[0])
	s = float64(val)
	if (data[1] & 0x80) == 0x80 {
		s /= 10
	}

	return
}

/*
Ускорение движения. Длина: 8 байт.
1-6 байты: не используются;
7-й байт: младший байт ускорения (0,01 м/ с2);
8-й байт:
биты 0-6      старший байт ускорения (0,01 м/с2),
бит 7         признак отрицательного ускорения
(тут не верно, только отриц ускорение)
*/

// получить ускорение в м/с2
func byteToAcceleration(data [8]byte) (a float64) {
	h := uint16(data[7]&0x7F) << 8
	val := h | uint16(data[6])
	accel := float64(val) / 100
	if (data[7] & 0x80) == 0x80 {
		accel *= (-1)
	}
	return
}

/* 5FCН сообщения о давлении - не реже 5 раз в секунду
Значения давления (все значения в 0,1 кг/см2). Длина: 5 байтов.
1-й байт – давление в тормозной магистрали;
2-й байт – давление в тормозном цилиндре;
3-й байт – давление в главном резервуаре;
4-й байт – давление в уравнительном резервуаре 1;
5-й байт – давление в уравнительном резервуаре 2.
*/

// получить давление ТМ, ТЦ, ГР в 0,1 кг/см2
func byteToPressure(data [8]byte) (tm, tc, gr float64) {

	tm = float64(data[0]) / 10
	tc = float64(data[1]) / 10
	gr = float64(data[2]) / 10

	return
}

// пробег от начала поездки в метрах, младшим байтом вперед.
func byteDistance(data [8]byte) (m uint32) {

	s := data[0:4] //слайс потому что функция не жрет массив
	m = binary.LittleEndian.Uint32(s)

	return
}

func byteToTimeBU(data [8]byte) (tm time.Time) {

	tm = time.Date(int((uint(data[0])<<8)|(uint(data[1]))), //год
		time.Month(data[2]),       //месяц
		int(data[3]),              //день
		int(data[4]),              //час
		int(data[5]),              //мин
		int(data[6]), 0, time.UTC) //секунды не учитываем

	return
}
