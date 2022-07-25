package main

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2/widget"
	"github.com/BurntSushi/toml"
)

// 3пв
var params = make(map[int]string)      // просто список параметров todo для всех БУ
var paramsValue = make(map[int]string) // значения из toml для подгрузки при старте

var paramEntry = make(map[int]*widget.Entry)

func declareParams() {
	params[2] = "Диаметр бандажа первой колёсной пары" // (мм)"
	paramsValue[2] = "1350"

	params[3] = "Диаметр бандажа второй колёсной пары" // (мм)"
	paramsValue[3] = "1350"

	params[4] = "Наличие МПМЭ" // названия тоже могут отличаться в разных бу, БУ известно заранее
	paramsValue[4] = "1"       // todo формировать для каждого блока свое

	params[5] = "Тип локомотива или электросекции"
	paramsValue[5] = "111"

	params[6] = "Номер локомотива или электросекции"
	paramsValue[6] = "1"

	params[7] = "Число зубьев датчика угла поворота"
	paramsValue[7] = "42"

	params[8] = "Верхний предел шкалы"
	paramsValue[8] = "100"

	params[9] = "Дискретность регистрации пути" //"Масштаб регистрации шкалы для БР-2М"
	paramsValue[9] = "100"

	params[10] = "Дискретность регистрации скорости" //"Дискретность регистрации скорости для БР-2М"
	paramsValue[10] = "1.0"

	params[11] = "Наличие БР-2М"
	paramsValue[11] = "0"

	params[12] = "Верхний предел измерения давления в ТЦ" // по 2 каналу"
	paramsValue[12] = "16"

	params[13] = "Признак наличия блока контроля"
	paramsValue[13] = "1"

	params[14] = "Уставка скорости V(ж)"
	paramsValue[14] = "45"

	params[15] = "Уставка скорости V(кж)"
	paramsValue[15] = "30"

	params[16] = "Уставка скорости V(упр)"
	paramsValue[16] = "10"

	params[17] = "Признак одной или двух кабин или МВПС"
	paramsValue[17] = "1"

	params[18] = "Код варианта системы АЛС"
	paramsValue[18] = "10"

	params[19] = "Признак наличия БУС"
	paramsValue[19] = "0"

	params[20] = "Кол-во метров для гребнесмазки"
	paramsValue[20] = "15"

	params[21] = "Наличие комплекса КВАРТА"
	paramsValue[21] = "0"

	params[22] = "Дискретность регистрации топлива"
	paramsValue[22] = "10"

	// params[23] = "Дата" нужно ли записывать не текущую дату?
	// params[24] = "Год"
	params[25] = "Количество дополнительных параметров"
	paramsValue[25] = "0"

	params[26] = "Количество знаков табельного номера"
	paramsValue[26] = "4"

}

/*
	todo
	1 Выводить сохраненное в файл при старте
	2 Считывать данные с БУ
	3 Записывать данные с формы в БУ

1
- взять данные из файла
- отобразить на экране как есть
toml -> entry
f сохранения в файл
f выгрузки из файла в форму

2
Перейти в режим обслуживания?
Считать данные из БУ в map
Вывести на экран полученное (или ошибку считывания)
Выйти из режима обслуживания?
map -> entry

3
Взять из формы значения
Проверить их
Записать в БУ
Вывести на экран результат записи ok/error

*/

// Conf s;dlgk
// type Conf struct {
// 	Title string
// 	Upp   upp `toml:"upp"`
// }

// type upp struct {
// 	data map[string]string
// 	// Band1 string
// 	// Band2 string
// 	// Diam  string
// }

//
// conf := new(Conf)
// if _, err := toml.DecodeFile("test.toml", conf); err != nil {
// 	fmt.Println(err)
// } else {
// 	fmt.Println(conf.Data.Diam)
// }
//

// var s struct {
// 	FOO struct {
// 		Passwords map[string]string
// 	}
// }

// // conf := new(Conf)
// _, err = toml.DecodeFile("upp.toml", &s)
// if err != nil {
// 	fmt.Println(err)
// }
// fmt.Printf("%v", s.FOO.Passwords["2"])

func getTomlUPP() (result map[int]string) {
	var err error
	var data struct {
		UPP struct {
			BU3pv map[string]string
		}
	}
	result = make(map[int]string)

	_, err = toml.DecodeFile("upp.toml", &data)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("%v", data.UPP.BU3pv["2"])

	for i, t := range data.UPP.BU3pv {
		val, _ := strconv.Atoi(i)
		result[val] = t
	}
	return
}

/*
func readToml(val map[int]string) {
	var data struct {
		UPP struct {
			BU3pv map[string]string
		}
	}

	for x, val := range val {
		data.UPP.BU3pv[fmt.Sprintf("%d", x)] = val
	}

	var buffer bytes.Buffer

	err := toml.NewEncoder(&buffer).Encode(&val)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("%v\n", buffer.String())
	}

	err = ioutil.WriteFile("debug.toml", buffer.Bytes(), 0644)
	if err != nil {
		fmt.Println(err)
	}
}*/
