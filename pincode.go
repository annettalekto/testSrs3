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

			attemptCounter := 0
			for {
				if bServiceModeBU4 {
					logInfo = fmt.Sprintf("Блок перешел в режим обслуживания (выход - перезагрузка).")
					ok = true
					break
				} else {
					time.Sleep(time.Second / 4)
					attemptCounter++
					if attemptCounter == 10 {
						logInfo += fmt.Sprintf(" Ответное сообщение не принято r\n")
						break
					}
				}
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
	ok = bServiceModeBU4
	return
}
