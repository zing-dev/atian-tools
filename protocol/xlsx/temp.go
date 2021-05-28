package xlsx

import (
	"context"
	"fmt"
	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/robfig/cron/v3"
	"github.com/zing-dev/atian-tools/log"
	"github.com/zing-dev/atian-tools/source/atian/dts"
	"os"
	"sync"
	"time"
)

const (
	LocalTimeFormat = "2006-01-02 15:04:05"
	EXT             = ".xlsx"
)

type Store struct {
	ctx    context.Context
	cancel context.CancelFunc

	Cron    *cron.Cron
	CronIds map[cron.EntryID]interface{}
	Temp    chan dts.ZonesTemp
	file
	Config Config
}

type file struct {
	sync.RWMutex
	*excelize.File
	sheets  map[string]int
	columns map[string]int
}

type Config struct {
	Host          string
	Dir           string
	name          string
	MinTempMinute byte //分钟
	MinSaveHour   byte //小时
}

func (o *Config) GetName() string {
	start := time.Now()
	duration := time.Duration(o.MinSaveHour) * time.Minute
	end := start.Add(duration)
	if duration == 0 {
		o.name = start.Format("2006-01-02")
	} else if duration < time.Hour {
		o.name = start.Format("2006-01-02-15-04") + "~" + end.Format("2006-01-02-15-04")
	} else if duration < time.Hour*24 {
		o.name = start.Format("2006-01-02-15") + "~" + end.Format("2006-01-02-15")
	} else {
		o.name = start.Format("2006-01-02") + "~" + end.Format("2006-01-02")
	}
	o.name += EXT
	log.L.Info(fmt.Sprintf("%s xlsx 文件名 %s", o.Host, o.name))
	return o.name
}

func New(ctx context.Context, config Config) *Store {
	ctx, cancel := context.WithCancel(ctx)
	s := &Store{
		ctx:    ctx,
		cancel: cancel,
		Config: config,
		file: file{
			sheets:  make(map[string]int),
			columns: make(map[string]int),
		},
		Temp:    make(chan dts.ZonesTemp, 2),
		Cron:    cron.New(cron.WithSeconds()),
		CronIds: map[cron.EntryID]interface{}{},
	}
	s.Write()
	return s
}

func (x *Store) Close() {
	for id := range x.CronIds {
		x.Cron.Remove(id)
	}
	x.Cron.Stop()
	x.cancel()
}

func (x *Store) Save() {
	x.File.DeleteSheet("Sheet1")
	err := x.File.Save()
	if err != nil {
		log.L.Error(fmt.Sprintf("保存主机 %s 的 xlsx 失败: %s", x.Config.Host, err))
	}
}

func (x *Store) New() {
	dir := fmt.Sprintf("%s/%s", x.Config.Dir, x.Config.Host)
	path := fmt.Sprintf("%s/%s", dir, x.Config.GetName())
	f, err := excelize.OpenFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			_, err := os.Open(dir)
			if os.IsNotExist(err) {
				err := os.MkdirAll(dir, 0777)
				if err != nil {
					log.L.Error("store", "创建保存温度更新文件夹失败")
				}
			}
			x.File = excelize.NewFile()
			x.File.Path = path
			x.sheets = make(map[string]int)
			x.columns = make(map[string]int)
		} else {
			log.L.Error(fmt.Sprintf("store: 打开 %s 失败", x.Config.name))
		}
	} else {
		x.File = f
	}
}

func (x *Store) write(channel string, sortZones dts.SortZones) {
	x.file.Lock()
	defer x.file.Unlock()
	if index, ok := x.sheets[channel]; ok {
		x.File.SetActiveSheet(index)
		column := columnId(x.columns[channel])
		x.File.SetCellValue(channel, fmt.Sprintf("%s%d", column, 1), time.Now().Format(LocalTimeFormat))
		x.File.SetColWidth(channel, column, column, 20)
		for k, v := range sortZones {
			if x.File.GetCellValue(channel, fmt.Sprintf("A%d", k+2)) == v.Name {
				x.File.SetCellValue(channel, fmt.Sprintf("%s%d", column, k+2), v.Temperature.Avg)
			}
		}
		x.columns[channel] += 1
	} else {
		x.sheets[channel] = x.File.NewSheet(channel)
		x.File.SetActiveSheet(x.sheets[channel])
		x.File.SetCellValue(channel, "A1", "防区")
		x.File.SetCellValue(channel, "B1", time.Now().Format(LocalTimeFormat))
		x.File.SetColWidth(channel, "A", "B", 20)
		for k, v := range sortZones {
			x.File.SetCellValue(channel, fmt.Sprintf("A%d", k+2), v.Name)
			x.File.SetCellValue(channel, fmt.Sprintf("B%d", k+2), v.Temperature.Avg)
		}
		x.columns[channel] = 67
	}
}

func (x *Store) Write() {
	//保存温度间隔的数据
	id, err := x.Cron.AddFunc(fmt.Sprintf("2 */%d * * * *", x.Config.MinTempMinute), func() {
		select {
		case temp := <-x.Temp:
			log.L.Info(fmt.Sprintf("开始保存主机 %s 的温度数据", x.Config.Host))
			x.New()
			zones := dts.SortZones(temp.Zones)
			dts.OrderedBy(func(p1, p2 *dts.Zone) bool {
				return p1.ChannelId < p2.ChannelId
			}, func(p1, p2 *dts.Zone) bool {
				return p1.Id < p2.Id
			}).Sort(zones)
			for _, zone := range zones.ChannelZones() {
				if len(zone) > 1 {
					log.L.Info(fmt.Sprintf("开始保存通道通道 %d 温度", zone[0].ChannelId))
					x.write(fmt.Sprintf("通道 %d ", zone[0].ChannelId), zone)
				}
			}
			x.Save()
			log.L.Info(fmt.Sprintf("保存主机 %s 的温度数据结束", x.Config.Host))
		}
	})
	if err != nil {
		return
	}
	x.CronIds[id] = struct{}{}

	if x.Config.MinSaveHour == 0 {
		id2, err := x.Cron.AddFunc(fmt.Sprintf("0 0 0 * * *"), func() {
			x.Config.GetName()
			x.sheets = make(map[string]int)
			x.columns = make(map[string]int)
		})
		if err != nil {
			return
		}
		x.CronIds[id2] = struct{}{}
	} else {
		//保存到文件的数据
		id2, err := x.Cron.AddFunc(fmt.Sprintf("0 0 */%d * * *", x.Config.MinSaveHour), func() {
			x.Config.GetName()
			x.sheets = make(map[string]int)
			x.columns = make(map[string]int)
		})
		if err != nil {
			return
		}
		x.CronIds[id2] = struct{}{}
		x.Cron.Start()
	}
}

func (x *Store) Store(temp dts.ZonesTemp) {
	select {
	case <-x.Temp:
	case x.Temp <- temp:
	default:
	}
}

func columnId(column int) string {
	column -= 64
	str := ""
	for column > 26 {
		i := column % 26
		if i == 0 {
			str = string(rune(64+26)) + str
			column = (column - 26) / 26
		} else {
			str = string(rune(64+i)) + str
			column = (column - i) / 26
		}
	}
	return string(rune(column+64)) + str
}
