// Code generated .* DO NOT EDIT.
package main

/*
	файл скрипта бу4
*/

// идентификаторы CAN25
const (
	SYS_DATA        = 0x313
	SYS_DATA_QUERY  = 0x70
	BU4_SYS_INFO    = 0x5C0 // Системная информация
	BU3P_QUERY_INFO = 0x5C1 // Запрос системной информации БУ-3П
	BU4_SET_PARAM   = 0x5C3 // Установка настроек
	BU4_CORR_TIME   = 0x5C7 // Коррекция времени
	// BU4_ERRORS    = 0x5C2 // Ошибки
)

const (
	// параметры (первый байт сообщения -- номер запрашиваемого признака) SYS_DATA_QUERY\SYS_DATA
	TAB_NUNBER   = 11
	ACCEL        = 101
	SOFT_VERSION = 110 // [1]=MAJOR [2]=MINOR [3]=PATCH [4]=номер в лоции

	// параметры BU4_SYS_INFO, BU3P_QUERY_INFO, BU4_SET_PARAM
	SERVICE_MODE = 0x2A // Пин-код/Обслуживание
	IPK_MODE     = 0x2B // Режим для ИПК
)
