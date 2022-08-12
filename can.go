package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/amdf/ixxatvci3/candev"
)

const (
	idTimeBU     = 0xC7
	idSpeed1     = 0x5E5
	idSpeed2     = 0x5E6
	idPressure   = 0x5FC
	idDistance   = 0x5C6
	idALS        = 0x50
	idCodeIF     = 0x5C5
	idBin        = 0x5F8
	idErrors     = 0x5C2
	idStatusBI   = 0x581 // словосостояние БИ
	idBI         = 0x580 // состояние клавиатуры
	idDigitalInd = 1     // fake
	idAddInd     = 2
	idSetTime    = 0x5C7
	idSetUPP     = 0x5C3
)

// устанавливать время (в режиме обслуживания)
func setTimeBU(h, m, s int) (err error) {
	var msg candev.Message

	dt := time.Now()
	y, _ := strconv.Atoi(dt.Format("2006"))
	month, _ := strconv.Atoi(dt.Format("1"))
	d, _ := strconv.Atoi(dt.Format("2"))

	msg.ID = uint32(idSetTime)
	msg.Rtr = false
	msg.Len = 8
	msg.Data = [8]byte{byte(y), byte(y >> 8), byte(month), byte(d), byte(h), byte(m), byte(s)} //5C7 -  07 E5 01 02 03 04 05 проверить: C7 - 07 E5 01 01 01 12 00

	err = can25.Send(msg)

	// проверить установку можно в рабочем режиме блока по сообщению 0xC7
	return
}

func setCurrentTimeBU() (err error) {

	dt := time.Now()
	h, _ := strconv.Atoi(dt.Format("15"))
	m, _ := strconv.Atoi(dt.Format("4"))
	s, _ := strconv.Atoi(dt.Format("5"))

	err = setTimeBU(h, m, s)

	return
}

// по сути для одного признака 10 Дискретность регистрации топлива для БР(0.5, 1.0, 2.0)
func setFloatVal(mod int, s string) (err error) {
	var f float64
	if f, err = strconv.ParseFloat(s, 64); err != nil {
		err = errors.New("setFloatVal(): значение \"" + s + "\" не переведено в float64")
		return
	}

	var sendMsg, receiveMsg candev.Message
	sendMsg.ID = idSetUPP
	sendMsg.Len = 5

	d1 := int(f)                      // целая
	d2 := int((f - float64(d1)) * 10) // дробная часть

	sendMsg.Data[0] = byte(mod)
	sendMsg.Data[1] = byte(d2 & 0xFF)
	sendMsg.Data[2] = byte((d2 >> 8) & 0xFF)
	sendMsg.Data[3] = byte(d1 & 0xFF)
	sendMsg.Data[4] = byte((d1 >> 8) & 0xFF)
	can25.Send(sendMsg)

	receiveMsg, err = can25.GetMsgByID(0x5C0, 2*time.Second)
	if err == nil {
		if sendMsg.Data != receiveMsg.Data {
			err = fmt.Errorf("setFloatVal(): значение признака не установлено: %X %X %X %X", sendMsg.Data[1], sendMsg.Data[2], sendMsg.Data[3], sendMsg.Data[4])
		}
	}

	return
}

// установить УПП int по CAN
// только в режиме обслуживания блока
func setIntVal(mod int, s string) (err error) {
	var val int
	if val, err = strconv.Atoi(s); err != nil {
		err = errors.New("setIntVal(): значение \"" + s + "\" не переведено в int")
		return
	}

	var sendMsg, receiveMsg candev.Message
	sendMsg.ID = idSetUPP
	sendMsg.Len = 5

	sendMsg.Data[0] = byte(mod)
	sendMsg.Data[1] = byte(val & 0xFF)
	sendMsg.Data[2] = byte((val >> 8) & 0xFF)
	sendMsg.Data[3] = byte((val >> 16) & 0xFF)
	sendMsg.Data[4] = byte((val >> 24) & 0xFF)
	can25.Send(sendMsg) // установить значение

	// поймать ответ, должен быть таким же как отправленное сообщение
	// это же сообщение можно запросить через 0x5C1, rtr = 1
	receiveMsg, err = can25.GetMsgByID(0x5C0, 2*time.Second)
	if err == nil {
		if sendMsg.Data != receiveMsg.Data {
			err = fmt.Errorf("setIntVal(): значение признака не установлено: %d", (int(sendMsg.Data[2])<<8 | int(sendMsg.Data[1])))
		}
	}

	return
}

//---------------------------------------- Индикатор ----------------------------------------//

// перевести байт полученный по САN с индикатора к строке (букве)
func byteIndToStr(b byte) (s string) {
	digits := map[byte]string{0xFF: " ", 0x82: "0", 0xCF: "1", 0x91: "2", 0x85: "3", 0xCC: "4", 0xA4: "5", 0xA0: "6", 0x8F: "7", 0x80: "8", 0x84: "9",
		0x02: "0.", 0x4F: "1.", 0x11: "2.", 0x05: "3.", 0x4C: "4.", 0x24: "5.", 0x20: "6.", 0x0F: "7.", 0x00: "8.", 0x04: "9."}

	s = digits[b]
	return
}

func strToByteInd(s string) (b byte) {
	bytes := map[string]byte{" ": 0xFF, "0": 0x82, "1": 0xCF, "2": 0x91, "3": 0x85, "4": 0xCC, "5": 0xA4, "6": 0xA0, "7": 0x8F, "8": 0x80, "9": 0x84,
		"0.": 0x02, "1.": 0x4F, "2.": 0x11, "3.": 0x05, "4.": 0x4C, "5.": 0x24, "6.": 0x20, "7.": 0x0F, "8.": 0x00, "9.": 0x04}
	b = bytes[s]
	return
}

/*
Установка основного индикатора. Длина: 8 байтов.
1-й байт: 01 – модификатор команды;
2-4 байты: значения для индикаторов;
5-8 байты зарезервированы (нули).
*/

// getDigitalIndicator Получить данные с цифрового индикатора
func byteToDigitalIndicator(data [8]byte) (str string) {

	if data[0] == 0x01 { // модификатор цифрового индикатора

		str = byteIndToStr(data[1]) + byteIndToStr(data[2]) + byteIndToStr(data[3])
		// if f, err = strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		// 	break
		// }
	}

	return
}

/*
Установка дополнительного индикатора. Длина: 8 байтов.
1-й байт: 02 – модификатор команды;
2-5 байты: значения для индикаторов;
6-8 байты зарезервированы (нули).
*/

// getAddIndicator Получить данные с дополнительного индикатора
func byteToAddIndicator(data [8]byte) (str string) {

	if data[0] == 0x02 { // модификатор

		str = byteIndToStr(data[1]) + byteIndToStr(data[2]) + byteIndToStr(data[3]) + byteIndToStr(data[4])
		// if f, err = strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil { // если значение на экране мигает, мы получаем пробелы всесто числа, тут еrr будет не nil
		// 	if msg.Data[1] == 0xFD { // минус, например, отрицательное ускорение
		// 		s = "-" + s
		// 		f *= -1
		// 	}
		// 	break
		// }
	}

	return
}

//----------------------------------------  Данные  ----------------------------------------//

/*
5C5H Состояние периода кодирования и кода рельсовой цепи. Длина: 3 байта.
1-й байт: период кодирования (16, 19, 60);
2-й байт: время после последнего полного периода (время сбоя) в секундах;
3-й байт: код рельсовой цепи (0 – сбой кода; 1 – код зеленого огня; 2 – код желтого с красным огня; 3 – код желтого огня).
*/

// получить состояние периода кодирования и кода рельсовой цепи
func byteToCodeIF(data [8]byte) (period, t, code int, str string) {

	period = int(data[0]) // 16 19 60
	t = int(data[1])
	code = int(data[2]) // 0 1 2 3
	switch code {
	case 0:
		str = "сбой кода"
	case 1:
		str = fmt.Sprintf("З %.1f (%d с)", float32(period)/10, t) // "1=З"
	case 2:
		str = fmt.Sprintf("КЖ %.1f (%d с)", float32(period)/10, t) // "2=КЖ"
	case 3:
		str = fmt.Sprintf("Ж %.1f (%d с)", float32(period)/10, t) // "3=Ж"
	}
	// fmt.Printf("Период кодирования: %d, код рельсовой цепи: %d, %v\n", period, code, err)

	return
}

// 0x50 Состояние сигналов АЛС; 8 байт
// 6-й байт: биты 0-3 коды огней светофора многозначной АЛС,
// Коды:  0 – белый, 1 - красный, 2 – КЖ, 3 – желтый, 4 – зеленый, 5-7 – нет огня
//

// получить код АЛС
func byteToALS(data [8]byte) (als int, str string) {

	als = int(data[5] & 0xF)
	switch als {
	case 0:
		str = "белый" // "0=Б"
	case 1:
		str = "красный" // "1=К"
	case 2:
		str = "КЖ" // "2=KЖ"
	case 3:
		str = "желтый" // "3=Ж"
	case 4:
		str = "зеленый" // "4=3"
	case 5, 6, 7:
		str = "нет"
	}
	// fmt.Printf("CAN Cигнал АЛС: %X\n", als)

	return
}

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

/*
(1) Ошибки БУ-3П. Длина: 3 байта, RTR=0.
1 байт: состояние ошибки (0 – ошибка исправлена, 1- ошибка возникла);
2..3 байты: номер ошибки в формате, принятом для комплексов КПД, младшим байтом вперед.
(2) Запрос состояния ошибки БУ-3П. Длина: 2 байта.
1..2 байты: номер ошибки в формате, принятом для комплексов КПД, младшим байтом вперед.
(3) Запрос ошибок БУ-3П. Длина: 0 байта, RTR=1.
Ответом на этот запрос будут переданы коды всех неисправленных ошибок.
5c2 rtr
5c2 01 58 02 -> возникла 0x258 = 600
*/

/*
Значения двоичных сигналов 5F8Н Длина: 4 байта.
2-й байт (не с нуля), бит 4 тяга (с нуля)
должна передаваться по каналу CAN не реже 1 раза в секунду
*/

// получть значение сигнала Тяги
func byteToBinSignal(data [8]byte) (str string) {

	// бит 0          Отпуск 1 каб.,
	// бит 1          Перекр. 1 каб.,
	// бит 2          Торм. 1 каб.,
	// бит 3          Экстр. 1 каб.,
	// бит 4          Отпуск 2 каб.,
	// бит 5          Перекр. 2 каб.,
	// бит 6          Торм. 2 каб.,
	// бит 7          Экстр. 2 каб.;

	// бит 0          ход вперед,
	// бит 1          ход назад,
	// бит 2          кабина 1 активна,
	// бит 3          кабина 2 активна,
	// бит 4          тяга,
	// бит 5          не используется,
	// бит 6          прохождение реперной точки (проезд шлейфа САУТ),
	// бит 7          не используется;

	if (data[1] & 0x01) == 0x001 {
		str = "движение вперед установлено"
	} else {
		str = "движение вперед..."
	}

	return
}
