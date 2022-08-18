package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// DataUPP upp
type DataUPP struct {
	// Number int
	Name  string
	Value string
	Hint  string
}

type descriptionType struct {
	NameBU           string
	Power            bool
	BandageDiameter1 uint32
	BandageDiameter2 uint32
	PressureLimit    float64
	NumberTeeth      uint32
	ScaleLimit       uint32
}

var gUPP = make(map[int]DataUPP) // все признаки
var gDevice descriptionType

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

	// из всех признаков выбираем те что используются в расчётах
	ival, err := strconv.Atoi(gUPP[2].Value)
	if err != nil {
		fmt.Printf("ОШИБКА. Значение УПП: \"%s\" не верно: %v\n", gUPP[2].Name, ival)
	}
	gDevice.BandageDiameter1 = uint32(ival)

	ival, err = strconv.Atoi(gUPP[3].Value)
	if err != nil {
		fmt.Printf("ОШИБКА. Значение УПП: \"%s\" не верно: %v\n", gUPP[3].Name, ival)
	}
	gDevice.BandageDiameter2 = uint32(ival)

	ival, err = strconv.Atoi(gUPP[7].Value)
	if err != nil {
		fmt.Printf("ОШИБКА. Значение УПП: \"%s\" не верно: %v\n", gUPP[7].Name, ival)
	}
	gDevice.NumberTeeth = uint32(ival)

	gDevice.PressureLimit, err = strconv.ParseFloat(gUPP[12].Value, 64)
	if err != nil {
		fmt.Printf("ОШИБКА. Значение УПП: \"%s\" не верно: %v\n", gUPP[12].Name, gUPP[12].Value)
	}

	ival, err = strconv.Atoi(gUPP[8].Value)
	if err != nil {
		fmt.Printf("ОШИБКА. Значение УПП: \"%s\" не верно: %v\n", gUPP[8].Name, ival)
	}
	gDevice.ScaleLimit = uint32(ival)

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

func checkValueUPP(value, hint string) (result bool) {
	fvalue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		fmt.Printf("Ошибка конвертации значения\n")
	}

	// 16 = "Диапазон значений [0 - 150]"
	if strings.Contains(hint, "Диапазон") {
		str := strings.TrimSuffix(hint, "]")
		inx := strings.IndexRune(str, '[')
		str = str[inx+1:]
		str = strings.ReplaceAll(str, " ", "")
		smin, smax, _ := strings.Cut(str, "-")
		min, err := strconv.ParseFloat(smin, 64)
		if err != nil {
			fmt.Printf("Ошибка получения значения из подсказки\n")
		}
		max, err := strconv.ParseFloat(smax, 64)
		if err != nil {
			fmt.Printf("Ошибка получения значения из подсказки\n")
		}
		if (fvalue >= min) && (fvalue <= max) {
			result = true
		} else {
			result = false
		}
	} else {
		// 17 = "Возможные значения [1, 2, 3]"
		str := strings.TrimSuffix(hint, "]")
		inx := strings.IndexRune(str, '[')
		str = str[inx+1:]
		str = strings.ReplaceAll(str, " ", "")
		sl := strings.Split(str, ",")
		for _, val := range sl {
			fv, err := strconv.ParseFloat(val, 64)
			if err != nil {
				fmt.Printf("Ошибка конвертации значения\n")
			}
			if fv == fvalue {
				result = true
				break
			}
		}
	}

	return
}

func writeUPPtoBU() (error, int) {
	var err error
	for number, upp := range gUPP {
		if number == 10 {
			err = setFloatVal(number, upp.Value)
		} else {
			err = setIntVal(number, upp.Value)
		}
		if err != nil {
			return err, number
		}
	}
	return nil, 0
}
