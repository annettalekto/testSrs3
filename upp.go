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
	Mod   int
	Name  string
	Value string
	Hint  string
}

type descriptionType struct { // todo имя
	NameBU           string
	Power            bool
	BandageDiameter1 uint32
	BandageDiameter2 uint32
	PressureLimit    float64
	NumberTeeth      uint32
	ScaleLimit       uint32
}

var gUPP = make(map[int]DataUPP) // все признаки todo все глобальные?
var gDevice descriptionType      // gBU.Name

func readUPPfromTOML() {
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

		upp.Mod = number
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
func writeUPPtoTOML() {
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

func (d DataUPP) checkValueUPP() (err error) {
	var ok bool
	hint := d.Hint

	fvalue, err := strconv.ParseFloat(d.Value, 64)
	if err != nil {
		fmt.Printf("Ошибка конвертации значения\n")
		err = fmt.Errorf("Введено неверное значение параметра %d «%s» = %s", d.Mod, d.Name, d.Value)
		return
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
			return fmt.Errorf("Ошибка получения значения из подсказки (файл errors.toml)")
		}
		max, err := strconv.ParseFloat(smax, 64)
		if err != nil {
			return fmt.Errorf("Ошибка получения значения из подсказки (файл errors.toml)")
		}
		if (fvalue >= min) && (fvalue <= max) {
			ok = true
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
				return fmt.Errorf("Ошибка получения значения из подсказки (файл errors.toml)")
			}
			if fv == fvalue {
				ok = true
				break
			}
		}
	}
	if !ok {
		err = fmt.Errorf("Введено неверное значение параметра: %d «%s»", d.Mod, d.Name)
	}

	return
}

func (d DataUPP) writeValue() (err error) {

	if d.Mod == 10 {
		err = setFloatVal(d.Mod, d.Value)
	} else {
		err = setIntVal(d.Mod, d.Value)
	}
	if err != nil {
		err = fmt.Errorf("Ошибка установки значения: %d «%s» = %s", d.Mod, d.Name, d.Value)
	}
	return
}

// только в режиме обслуживания
func writeUPPtoBU() (err error) {
	for _, upp := range gUPP {
		if err = upp.writeValue(); err != nil {
			return
		}
	}
	return
}
