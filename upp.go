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
// (значения УПП сохраняются каждый раз при записи УПП в БУ, те "последние записанные")
// сохранить в gUPP
// запускать функцию первой
func readParamFromTOML() (mapupp map[int]DataUPP, err error) {
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

	mapupp = make(map[int]DataUPP)

	for i := range data.UPP.Name {
		var upp DataUPP
		number, _ := strconv.Atoi(i)

		upp.Mod = number
		upp.Name = data.UPP.Name[i]
		upp.Value = data.UPP.Value[i]
		upp.Hint = data.UPP.Hint[i]
		mapupp[number] = upp
	}
	// err = refreshDataBU()

	return
}

// записать текущие УПП (gUPP) в файл
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

	fileName := ".\\toml\\errors.toml"
	_, err = toml.DecodeFile(fileName, &data)
	if err != nil {
		fmt.Println(err)
	}
	return data.Errors.Description[sCode]
}

type configType struct {
	Name          string
	ProgramName   string
	Version       string
	Build         int
	Year          string
	Icon          string
	Theme         string
	DeviceVariant OptionsBU
}

func getFyneAPP() (data configType) {
	var err error
	var data1 struct {
		Details struct {
			Icon    string
			Name    string
			Version string
			Build   int
		}
	}
	fileName := ".\\FyneAPP.toml"
	_, err = toml.DecodeFile(fileName, &data1)
	if err != nil {
		fmt.Println(err)
	}

	var data2 struct {
		Details struct {
			ProgramName   string
			Year          string
			Theme         string
			DeviceVariant int
		}
	}
	fileName = ".\\config.toml"
	_, err = toml.DecodeFile(fileName, &data2)
	if err != nil {
		fmt.Println(err)
	}

	data.Name = data1.Details.Name
	data.ProgramName = data2.Details.ProgramName
	data.Version = data1.Details.Version
	data.Build = data1.Details.Build
	data.Year = data2.Details.Year
	data.Icon = data1.Details.Icon
	data.Theme = data2.Details.Theme
	data.DeviceVariant = OptionsBU(data2.Details.DeviceVariant)

	return
}

func writeFyneAPP(data configType) {
	f, err := os.Create(".\\config.toml")
	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()
	f.WriteString("[Details]\n")
	f.WriteString(fmt.Sprintf("ProgramName = \"%s\"\n", data.ProgramName))
	f.WriteString(fmt.Sprintf("Year = \"%s\"\n", data.Year))
	f.WriteString(fmt.Sprintf("Theme = \"%s\"\n", data.Theme))
	f.WriteString(fmt.Sprintf("DeviceVariant = %d", data.DeviceVariant))
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

// проверить на корректность и обновить данные, для расчета и установки параметров
func refreshDataBU() (err error) {

	i := 2
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

	if gBU.Variant == BU4 {
		gBU.PressureLimit = 10
	} else {
		i = 12
		gBU.PressureLimit, err = strconv.ParseFloat(gUPP[i].Value, 64)
		if err != nil {
			err = fmt.Errorf("ОШИБКА. Значение УПП: \"%s\" не верно: %v", gUPP[i].Name, gUPP[i].Value)
		}
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

	// БУ 4
	if gBU.Variant == BU4 {
		i = 40
		ival, err = strconv.Atoi(gUPP[i].Value)
		if err != nil {
			err = fmt.Errorf("ОШИБКА. Значение УПП: \"%s\" не верно: %v", gUPP[i].Name, ival)
		}
		gBU.NumberDUP = ival

		i = 41
		ival, err = strconv.Atoi(gUPP[i].Value)
		if err != nil {
			err = fmt.Errorf("ОШИБКА. Значение УПП: \"%s\" не верно: %v", gUPP[i].Name, ival)
		}
		gBU.NumberDD = ival
	}

	return
}

func checkUPP() (ok bool) {
	// дополнительные проверки UPP todo
	ok = true
	return
}
