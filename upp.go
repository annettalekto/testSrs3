package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/BurntSushi/toml"
)

// DataUPP upp
type DataUPP struct {
	// Number int
	Name  string
	Value string
	Hint  string
}

var gUPP = make(map[int]DataUPP)

func getTomlUPP() {
	var err error
	var data struct {
		UPP struct {
			Name  map[string]string
			Value map[string]string
			Hint  map[string]string
		}
	}
	_, err = toml.DecodeFile("upp.toml", &data)
	if err != nil {
		fmt.Println(err)
	}

	for i := range data.UPP.Name {
		var upp DataUPP
		number, _ := strconv.Atoi(i)
		upp.Name = data.UPP.Name[i]
		upp.Value = data.UPP.Value[i]
		upp.Hint = data.UPP.Hint[i]
		gUPP[number] = upp
	}
	return
}

// записать текущие УПП в файл
func writeTomlUPP() {
	f, err := os.Create("upp.toml")
	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()

	var temp []int
	for v := range gUPP {
		temp = append(temp, v)
	}
	sort.Ints(temp)

	f.WriteString("#БУ-3ПВ\n\n")
	f.WriteString("[UPP.Name]\n")
	for _, number := range temp {
		f.WriteString(fmt.Sprintf("%d = \"%s\"\n", number, gUPP[number].Name))
	}
	f.WriteString("\n[UPP.Value]\n")
	for _, number := range temp {
		f.WriteString(fmt.Sprintf("%d = \"%s\"\n", number, gUPP[number].Value))
	}
	f.WriteString("\n[UPP.Hint]\n")
	for _, number := range temp {
		f.WriteString(fmt.Sprintf("%d = \"%s\"\n", number, gUPP[number].Hint))
	}
}

func getErrorDescription(sCode string) string {
	var err error
	var data struct {
		Errors struct {
			Description map[string]string
		}
	}

	_, err = toml.DecodeFile("errors.toml", &data)
	if err != nil {
		fmt.Println(err)
	}
	return data.Errors.Description[sCode]
}

/*func debugDeclareParams() {
	params[2] = "Диаметр бандажа первой колёсной пары" // (мм)"
	paramsValue[2] = "1350"
	hints[2] = "Возможные значения 600 — 1350"

	params[3] = "Диаметр бандажа второй колёсной пары" // (мм)"
	paramsValue[3] = "1350"
	hints[3] = "Возможные значения 600 — 1350"

	params[4] = "Наличие МПМЭ"            // названия тоже могут отличаться в разных бу, БУ известно заранее
	paramsValue[4] = "1"                  // todo формировать для каждого блока свое
	hints[4] = "Возможные значения: 0, 1" // подсказки тоже или считать из toml

	params[5] = "Тип локомотива или электросекции"
	paramsValue[5] = "111"
	hints[5] = "Возможные значения 111 - 999"

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
}*/
