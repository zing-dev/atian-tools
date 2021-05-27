package beida_bluebird

import (
	"encoding/json"
	"fmt"
	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/zing-dev/atian-tools/log"
	"strconv"
	"strings"
)

type Map struct {
	Id         uint32
	Name       string //防区名 根据防区的物理坐标的规则
	Code       string //根据防区名称的规则生成,或根据防区的坐标生成
	Controller byte   //控制器号
	Loop       byte   //回路号
	Part       byte   //部位号
	PartType   byte   //部件类型
}

func (m *Map) String() string {
	return fmt.Sprintf("Id: %d ,防区名: %s ,防区编码: %s ,控制器号: %d ,回路号: %d ,部位号: %d ,部件类型: %d ", m.Id, m.Name, m.Code, m.Controller, m.Loop, m.Part, m.PartType)
}

func (m *Map) JSON() string {
	data, _ := json.Marshal(m)
	return string(data)
}

type Maps map[uint32][]*Map

// Key Controller PartType Loop Part
// 00000000 00000000 00000000 00000000
func (m *Map) Key() uint32 {
	m.Id = uint32(m.Controller)<<24 | uint32(m.PartType)<<16 | uint32(m.Loop)<<8 | uint32(m.Part)
	return m.Id
}

func (m Maps) Get(index uint32) []*Map {
	if v, ok := m[index]; ok {
		return v
	}
	log.L.Error("获取映射数据的索引失败")
	return nil
}

func (m Maps) Load(filename string) {
	file, err := excelize.OpenFile(filename)
	if err != nil {
		log.L.Fatal("打开XLSX失败: ", err)
		return
	}
	rows := file.GetRows(file.GetSheetName(file.GetActiveSheetIndex()))
	if len(rows) < 4 {
		log.L.Fatal("XLSX最少四行数据")
		return
	}
	log.L.Info("⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇防区和串口设备部件映射注意⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇")
	log.L.Info("→ 第一行必须是防区名,防区编码,控制器号,回路号,部位号,部件类型")
	log.L.Info("→ 第一列必须是防区名")
	log.L.Info("→ 第二列必须是防区名唯一编码")
	log.L.Info("→ 第三列必须是控制器号,必须和设备上的控制器号对应,范围 0~255")
	log.L.Info("→ 第四列必须是回路号,必须和设备上的回路号对应范围,范围 0~255")
	log.L.Info("→ 第五列必须是部位号,必须和设备上的部位号对应范围,范围 0~255")
	log.L.Info("→ 第六列必须是部件类型,必须和设备上部件类型的对应范围,范围 1~255")
	log.L.Info("⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆防区和串口设备部件映射注意⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆")
	log.L.Info("开始读取XLSX文件...")
	show := map[int]*Map{}
	for k, row := range rows[1:] {
		if len(row) >= 5 {
			controller := 1
			controller, err = strconv.Atoi(strings.TrimSpace(row[2]))
			if err != nil {
				log.L.Error(fmt.Sprintf("解析 %d 控制器号 %s 失败: %s", k+1, row[2], err))
				continue
			}
			loop, err := strconv.Atoi(strings.TrimSpace(row[3]))
			if err != nil {
				log.L.Error(fmt.Sprintf("解析 %d 回路号 %s 失败: %s", k+1, row[3], err))
				continue
			}
			part, err := strconv.Atoi(strings.TrimSpace(row[4]))
			if err != nil {
				log.L.Error(fmt.Sprintf("解析 %d 部位号 %s 失败: %s", k+1, row[4], err))
				continue
			}
			partType := int(PartTypeSmokeSensation)
			partType, err = strconv.Atoi(row[5])
			if err != nil {
				log.L.Error(fmt.Sprintf("解析 %d 部件类型失败: %s", k+1, err))
				continue
			}
			item := &Map{
				Name:       row[0],
				Code:       row[1],
				Controller: byte(controller),
				Loop:       byte(loop),
				Part:       byte(part),
				PartType:   byte(partType),
			}
			m[item.Key()] = append(m[item.Key()], item)
			if k == 0 || k == len(rows)/3 || k == len(rows)-2 {
				show[k] = item
			}
		} else {
			log.L.Error(fmt.Sprintf("第 %d 行数据列数不匹配", k+1))
		}
	}
	for k, v := range show {
		log.L.Info(fmt.Sprintf("第 %d 个防区: %s", k+1, v.String()))
	}
	log.L.Info(fmt.Sprintf("读取完成,共有 %d 个防区", len(m)))
}
