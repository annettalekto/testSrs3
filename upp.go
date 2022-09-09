package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

var gUPP = make(map[int]DataUPP)

// DataUPP upp
type DataUPP struct {
	Mod   int
	Name  string
	Value string
	Hint  string
}

// Прочиать УПП из toml: имена, значения УПП, подсказки и граничные значения
// сохранить в gUPP
// запускать функцию первой
func readParamFromTOML() (err error) {
	var data struct {
		UPP struct {
			Name  map[string]string
			Value map[string]string
			Hint  map[string]string
		}
	}
	b := getNameTOML()
	_, err = toml.DecodeFile(b, &data)
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
	err = refreshDataBU()

	return
}

// записать текущие УПП в файл
func writeParamToTOML() {
	f, err := os.Create(getNameTOML())
	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()

	var temp []int
	for v := range gUPP {
		temp = append(temp, v)
	}
	sort.Ints(temp)

	f.WriteString(fmt.Sprintf("#%s\n\n", gBU.Name))
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

	if d.Mod == 3 {
		err = setIntVal(2, d.Value) // бандаж 1
		err = setIntVal(3, d.Value) // бандаж 2
	}
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

func refreshDataBU() (err error) {
	// из всех признаков выбираем те что используются в расчётах и установках
	i := 3
	ival, err := strconv.Atoi(gUPP[i].Value)
	if err != nil {
		err = fmt.Errorf("ОШИБКА. Значение УПП: \"%s\" не верно: %v", gUPP[i].Name, ival)
	}
	gBU.BandageDiameter = uint32(ival)

	i = 7
	ival, err = strconv.Atoi(gUPP[i].Value)
	if err != nil {
		err = fmt.Errorf("ОШИБКА. Значение УПП: \"%s\" не верно: %v", gUPP[i].Name, ival)
	}
	gBU.NumberTeeth = uint32(ival)

	i = 12 // todo переинициализация этого
	gBU.PressureLimit, err = strconv.ParseFloat(gUPP[i].Value, 64)
	if err != nil {
		err = fmt.Errorf("ОШИБКА. Значение УПП: \"%s\" не верно: %v", gUPP[i].Name, gUPP[i].Value)
	}

	i = 8
	ival, err = strconv.Atoi(gUPP[i].Value)
	if err != nil {
		err = fmt.Errorf("ОШИБКА. Значение УПП: \"%s\" не верно: %v", gUPP[i].Name, ival)
	}
	gBU.ScaleLimit = uint32(ival)

	i = 14
	ival, err = strconv.Atoi(gUPP[i].Value)
	if err != nil {
		err = fmt.Errorf("ОШИБКА. Значение УПП: \"%s\" не верно: %v", gUPP[i].Name, ival)
	}
	gBU.RelayY = ival

	i = 15
	ival, err = strconv.Atoi(gUPP[i].Value)
	if err != nil {
		err = fmt.Errorf("ОШИБКА. Значение УПП: \"%s\" не верно: %v", gUPP[i].Name, ival)
	}
	gBU.RelayRY = ival

	i = 16
	ival, err = strconv.Atoi(gUPP[i].Value)
	if err != nil {
		err = fmt.Errorf("ОШИБКА. Значение УПП: \"%s\" не верно: %v", gUPP[i].Name, ival)
	}
	gBU.RelayU = ival

	return
}

func checkUPP() (ok bool) {
	// дополнительные проверки UPP todo
	ok = true
	return
}
