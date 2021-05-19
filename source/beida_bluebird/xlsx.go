package beida_bluebird

import (
	"atian.tools/log"
	"fmt"
	"github.com/360EntSecGroup-Skylar/excelize"
	"strconv"
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

type Maps map[uint32]*Map

// Key Controller PartType Loop Part
// 00000000 00000000 00000000 00000000
func (m *Map) Key() uint32 {
	m.Id = uint32(m.Controller)<<24 | uint32(m.PartType)<<16 | uint32(m.Loop)<<8 | uint32(m.Part)
	return m.Id
}

func (m Maps) Get(index uint32) *Map {
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
	if len(rows) <= 1 {
		log.L.Fatal("XLSX最少两行数据")
		return
	}
	log.L.Info("⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇防区和串口设备部件映射注意⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇⬇")
	log.L.Info("→ 第一行必须是防区名,防区编码,控制器号,回路号,部位号,部件类型")
	log.L.Info("→ 第一列必须是防区名")
	log.L.Info("→ 第二列必须是防区名唯一编码")
	log.L.Info("→ 第三列必须是控制器号,可以为空,默认为1,范围 1~255")
	log.L.Info("→ 第四列必须是回路号,范围 1~255")
	log.L.Info("→ 第五列必须是部位号,范围 1~255")
	log.L.Info("→ 第六列必须是部件类型,范围 可以为空,默认为1,1~255")
	log.L.Info("⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆防区和串口设备部件映射注意⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆⬆")
	for k, row := range rows[1:] {
		if len(row) >= 5 {
			controller := 1
			controller, err = strconv.Atoi(row[2])
			if err != nil {
				log.L.Error(fmt.Sprintf("解析 %d 控制器号失败: %s", k+1, err))
				continue
			}
			loop, err := strconv.Atoi(row[3])
			if err != nil {
				log.L.Error(fmt.Sprintf("解析 %d 回路号失败: %s", k+1, err))
				continue
			}
			part, err := strconv.Atoi(row[4])
			if err != nil {
				log.L.Error(fmt.Sprintf("解析 %d 部位号号失败: %s", k+1, err))
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
			m[item.Key()] = item
		} else {
			log.L.Error(fmt.Sprintf("第 %d 行数据列数不匹配", k+1))
		}
	}
	log.L.Info(fmt.Sprintf("读取完成,共有 %d 个防区", len(m)))
}
