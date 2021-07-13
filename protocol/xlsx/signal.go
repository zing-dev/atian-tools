package xlsx

import (
	"fmt"
	"github.com/360EntSecGroup-Skylar/excelize/v2"
	"github.com/robfig/cron/v3"
	"github.com/zing-dev/atian-tools/log"
	"github.com/zing-dev/atian-tools/source/atian/dts"
	"github.com/zing-dev/atian-tools/source/device"
	"os"
	"strings"
	"sync"
	"time"
)

type SignalStore struct {
	wg      sync.WaitGroup
	queue   chan dts.ChannelSignal
	config  *Config
	path    string
	incr    map[int32]int
	sheets  map[int32]int
	cron    *cron.Cron
	cronIds map[cron.EntryID]struct{}
	lock    sync.RWMutex
}

func NewSignalStore(cfg *Config) *SignalStore {
	s := &SignalStore{
		queue:   make(chan dts.ChannelSignal, 1),
		config:  cfg,
		incr:    make(map[int32]int),
		sheets:  make(map[int32]int),
		cronIds: make(map[cron.EntryID]struct{}),
		cron:    cron.New(cron.WithSeconds()),
	}
	s.run()
	return s
}

func (s *SignalStore) Store(data dts.ChannelSignal) {
	select {
	case s.queue <- data:
	default:
	}
}

func (s *SignalStore) Close() {
	s.wg.Wait()
	for id := range s.cronIds {
		s.cron.Remove(id)
	}
	s.cron.Stop()
}

func (s *SignalStore) run() {
	s.rename()
	//保存温度间隔的数据
	if id, err := s.cron.AddFunc(fmt.Sprintf("0 */%d * * * *", s.config.MinTempMinute), s.consumer); err != nil {
		log.L.Info(err)
		return
	} else {
		s.cronIds[id] = struct{}{}
	}
	if s.config.MinSaveHour == 0 {
		if id, err := s.cron.AddFunc(fmt.Sprintf("0 0 0 * * *"), s.rename); err != nil {
			return
		} else {
			s.cronIds[id] = struct{}{}
		}
	} else {
		//保存到文件的数据
		if id, err := s.cron.AddFunc(fmt.Sprintf("0 0 */%d * * *", s.config.MinSaveHour), s.rename); err != nil {
			return
		} else {
			s.cronIds[id] = struct{}{}
		}
	}
	//每日12点执行一次
	if id, err := s.cron.AddFunc("0 0 12 * * *", func() {
		dir := fmt.Sprintf("%s/%s", s.config.Dir, s.config.Host)
		last := time.Now().AddDate(-1, 0, 0).Format(device.LocalDateFormat)
		dirs, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, entry := range dirs {
			if strings.HasPrefix(entry.Name(), last) {
				name := fmt.Sprintf("%s/%s", dir, entry.Name())
				if err := os.Remove(name); err != nil {
					log.L.Error(fmt.Sprintf("删除设备 %s 历史文件 %s 失败: %s", s.config.Host, name, err))
				} else {
					log.L.Info(fmt.Sprintf("删除设备 %s 历史文件 %s", s.config.Host, name))
				}
			}
		}
	}); err != nil {
		return
	} else {
		s.cronIds[id] = struct{}{}
	}

	s.cron.Start()
	log.L.Info("通道温度信号更新XLSX 定时器开始后台运行...")
}

func (s *SignalStore) consumer() {
	select {
	case data := <-s.queue:
		s.wg.Add(1)
		log.L.Info(fmt.Sprintf("开始保存设备 %s 通道 %d 温度信号,数量 %d", s.config.Host, data.ChannelId, len(data.Signal)))
		s.process(data)
		log.L.Info(fmt.Sprintf("保存主机 %s 的温度信号数据结束", s.config.Host))
	default:
	}
}

func (s *SignalStore) rename() {
	s.lock.Lock()
	defer s.lock.Unlock()
	dir := fmt.Sprintf("%s/%s", s.config.Dir, s.config.Host)
	_, err := os.Open(dir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0777)
		if err != nil {
			log.L.Error("创建保存通道温度信号更新文件夹失败")
		}
	}
	s.incr = make(map[int32]int)
	s.sheets = make(map[int32]int)
	s.path = fmt.Sprintf("%s/%s", dir, s.config.GetName())
}

func (s *SignalStore) process(data dts.ChannelSignal) {

	var (
		file *excelize.File
		err  error
	)
	defer s.wg.Done()

	s.lock.Lock()
	defer s.lock.Unlock()

	file, err = excelize.OpenFile(s.path)
	if err != nil {
		file = excelize.NewFile()
		file.Path = s.path
		s.incr = make(map[int32]int)
		s.sheets = make(map[int32]int)
	}

	sheetName := fmt.Sprintf("通道 %d", data.ChannelId)

	if n, ok := s.sheets[data.ChannelId]; ok {
		file.SetActiveSheet(n)

		column, _ := excelize.ColumnNumberToName(s.incr[data.ChannelId])

		file.SetCellValue(sheetName, fmt.Sprintf("%s1", column), data.CreatedAt.String())
		file.SetColWidth(sheetName, column, column, 20)

		style, _ := file.NewStyle(&excelize.Style{
			Alignment: &excelize.Alignment{Horizontal: "right"},
		})
		file.SetCellStyle(sheetName, fmt.Sprintf("%s1", column), fmt.Sprintf("%s1", column), style)

		for i, f := range data.Signal {
			file.SetCellValue(sheetName, fmt.Sprintf("%s%d", column, i+2), f)
		}

		s.incr[data.ChannelId]++

	} else {
		sl := file.GetSheetList()
		var n = 0
		if len(sl) == 1 && sl[0] == "Sheet1" {
			n = file.GetActiveSheetIndex()
			file.SetActiveSheet(n)
			file.SetSheetName("Sheet1", sheetName)
		} else {
			n = file.NewSheet(sheetName)
		}

		s.sheets[data.ChannelId] = n

		file.SetCellStr(sheetName, "A1", "时间")
		file.SetCellValue(sheetName, "B1", data.CreatedAt.String())
		file.SetColWidth(sheetName, "A", "B", 20)
		style, _ := file.NewStyle(&excelize.Style{
			Alignment: &excelize.Alignment{Horizontal: "right"},
		})
		file.SetCellStyle(sheetName, "A1", "A1", style)
		file.SetCellStyle(sheetName, "B1", "B1", style)

		var total float32
		for i, f := range data.Signal {

			file.SetCellValue(sheetName, fmt.Sprintf("A%d", i+2), total)
			file.SetCellValue(sheetName, fmt.Sprintf("B%d", i+2), f)

			total += data.RealLength
		}

		s.incr[data.ChannelId] = 3
	}

	file.Save()

}
