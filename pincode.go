package main

/*
#include "crc.h"
*/
import "C"
import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/amdf/ixxatvci3/candev"
)

// Расчет пин для входа в режим обслуживания БУ-4
// из даты полученной из БУ (год без первых двух цифор!!
// -- так храниться год в памяти БУ). Пример:
// s := updatePinCode("01", "01", "00")
// fmt.Println(s)// 6468
func updatePinCode(uDay, uMonth, uYear uint64) (pincode uint32, err error) {

	if !((uDay >= 1 && uDay <= 31) &&
		(uMonth >= 1 && uMonth <= 12) &&
		(uYear >= 0 && uYear <= 2999)) {
		err = errors.New("updatePinCode(): некорректные входные данные")
		return
	}

	buf := []uint8{
		uint8(uDay & 0xFF),
		uint8(uMonth & 0xFF),
		uint8(uYear & 0xFF),
	}

	crc := C.pinCodeFromDate(C.uchar(buf[0]), C.uchar(buf[1]), C.uchar(buf[2]))

	digit1 := (crc & 0xFF) % 10
	digit2 := (crc >> 8 & 0xFF) % 10
	digit3 := (crc >> 16 & 0xFF) % 10
	digit4 := (crc >> 24 & 0xFF) % 10

	pincode = uint32(digit1*1000 + digit2*100 + digit3*10 + digit4)
	// pinCodeText = fmt.Sprintf("Пин-код: %04d", pincode)

	return pincode, err
}

func setServiceModeBU4() (ok bool, logInfo string) {

	msg := candev.Message{ID: BU4_SET_PARAM, Len: 5} // id установки УПП

	if t, err := canGetTimeBU(); err == nil {
		if pin, err := updatePinCode(uint64(t.Day()), uint64(t.Month()), uint64(t.Year())-2000); err == nil {

			result := make([]byte, 4)
			binary.LittleEndian.PutUint32(result, pin)
			copy(msg.Data[1:], result)
			msg.Data[0] = SERVICE_MODE

			can25.Send(msg)
			logInfo = "Пин-код отправлен."

			// делаем проверку режима
			if msg, err = can25.GetMsgByID(BU4_SYS_INFO, 2*time.Second); err == nil {
				if msg.Data[0] == SERVICE_MODE && msg.Data[1] == 1 {
					logInfo = fmt.Sprintf("Блок перешел в режим обслуживания (выход - перезагрузка).")
					ok = true
				} else {
					logInfo += fmt.Sprintf(" Блок не перешел в режим обслуживания или сообщение не принято (сообщение: %X %X)\r\n", msg.Data[0], msg.Data[1])
				}
			} else {
				logInfo += fmt.Sprintf(" Ответное сообщение не принято, err: %v\r\n", err)
			}
		} else {
			logInfo = fmt.Sprintf("Пин-код не сгенерирован, err: %v\r\n", err)
		}
	} else {
		logInfo = fmt.Sprintf("Пин-код не сгенерирован (не получено время БУ, err: %v)\r\n", err)
	}

	fmt.Println(logInfo)

	return
}

func isServiceModeBU4() (ok bool) {
	var msg candev.Message

	msg.ID = BU3P_QUERY_INFO
	msg.Data[0] = SERVICE_MODE
	can25.Send(msg)

	if msg, err := can25.GetMsgByID(BU4_SYS_INFO, 2*time.Second); err == nil {
		if msg.Data[0] == SERVICE_MODE && msg.Data[1] == 1 {
			// logInfo = fmt.Sprintf("Блок перешел в режим обслуживания.")
			ok = true
		}
	}
	return
}
