package beida_bluebird

import (
	"atian.tools/log"
	"bytes"
	"context"
	"errors"
	"go.bug.st/serial"
	"time"
)

const (
	// PassiveQueryInterval  被动模式,外部查询命令间隔为1秒。
	PassiveQueryInterval = time.Second

	CodeStart = 0x82
	CodeEnd   = 0x83

	// DataLength the data's length must be 26
	DataLength = 26

	// PartTypeIO 输入输出模块
	PartTypeIO byte = 13
	// PartTypeSmokeSensation 感烟
	PartTypeSmokeSensation byte = 21

	// CmdControllerFailure 控制器故障
	CmdControllerFailure byte = 0xEF
	// CmdAlarm0A 火警
	CmdAlarm0A byte = 0x0A
	// CmdAlarm80 火警
	CmdAlarm80 byte = 0x80
	// CmdFailure 故障
	CmdFailure byte = 0x81
)

type App struct {
	ctx    context.Context
	cancel context.CancelFunc

	Name    string
	Version string
	Maps    Maps
	Serial  serial.Port

	cache  *bytes.Buffer
	config *Config

	protocol     *Protocol
	chanProtocol chan *Protocol
}

type Config struct {
	Port    string
	MapFile string
}

type Protocol struct {
	Cmd        byte //报警命令
	Controller byte //控制器号
	Loop       byte //回路号
	Part       byte //部位号
	PartType   byte //部件类型
	Year       byte //时间年
	Month      byte //时间月
	Day        byte //时间日
	Hour       byte //时间时
	Minute     byte //时间分
	Second     byte //时间秒
}

func (p *Protocol) IsCmdAlarm() bool {
	return p.Cmd == CmdAlarm0A || p.Cmd == CmdAlarm80
}

func (p *Protocol) IsCmdFailure() bool {
	return p.Cmd == CmdFailure
}

func (p *Protocol) IsTypeIOModule() bool {
	return p.PartType == PartTypeIO
}

func (p *Protocol) IsTypeSmokeSensation() bool {
	return p.PartType == PartTypeSmokeSensation
}

// Encode 编码:除首尾外,其余为一个单字节拆成两个字节组成,高位在前,低位在后,每个单字节各自加上0x30
func (p *Protocol) Encode() [DataLength]byte {
	now := time.Now()
	p.Year = byte(now.Year() - 2000)
	p.Month = byte(now.Month())
	p.Day = byte(now.Day())
	p.Hour = byte(now.Hour())
	p.Minute = byte(now.Minute())
	p.Second = byte(now.Second())
	var (
		data        = [DataLength]byte{}
		values      = []byte{p.Cmd, p.Controller, p.Loop, p.Part, p.PartType, p.Year, p.Month, p.Day, p.Hour, p.Minute, p.Second}
		sum    byte = 0
	)
	data[0] = CodeStart
	data[DataLength-1] = CodeEnd

	for k, value := range values {
		data[k*2+1] = (value-value%0x10)/0x10 + 0x30
		data[k*2+2] = value%0x10 + 0x30
		sum += value
	}
	data[23] = (sum-sum%0x10)/0x10 + 0x30
	data[24] = sum%0x10 + 0x30
	return data
}

// Decode 解码:首尾数据为单个字节,其余位为两个单字节各自减去0x30后,高位在前低位在后合并为一个单字节
func (p *Protocol) Decode(data [DataLength]byte) error {
	var (
		values      = make([]byte, 11)
		sum    byte = 0
		sign   byte = 0
	)

	for k := range values {
		values[k] = (data[k*2+1]-0x30)*0x10 + (data[k*2+2] - 0x30)
		sum += values[k]
	}
	sign = (data[23]-0x30)*0x10 + (data[24] - 0x30)
	if sum&sign != sign {
		return errors.New("sum check not equal sign")
	}
	p.Cmd = values[0]
	p.Controller = values[1]
	p.Loop = values[2]
	p.Part = values[3]
	p.PartType = values[4]
	p.Year = values[5]
	p.Month = values[6]
	p.Day = values[7]
	p.Hour = values[8]
	p.Minute = values[9]
	p.Second = values[10]
	return nil
}

// DateTime DateTime
func (p *Protocol) DateTime() time.Time {
	return time.Date(int(p.Year)+2000, time.Month(int(p.Month)), int(p.Day), int(p.Hour), int(p.Minute), int(p.Second), 0, time.Local)
}

func New(ctx context.Context, config *Config) *App {
	ctx, cancel := context.WithCancel(ctx)
	return &App{
		ctx:          ctx,
		cancel:       cancel,
		Name:         "JBF293K 接口卡RS232/485",
		Version:      "1.3",
		Maps:         map[uint32]*Map{},
		cache:        new(bytes.Buffer),
		config:       config,
		protocol:     new(Protocol),
		chanProtocol: make(chan *Protocol, 10),
	}
}

// Run 运行App应用
func (a *App) Run() {
	a.Maps.Load(a.config.MapFile)
	port, err := serial.Open(a.config.Port, &serial.Mode{
		BaudRate: 9600,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		log.L.Fatal("open serial err: ", err)
		return
	}

	a.Serial = port
	go a.read()
}

func (a *App) read() {
	buffer := make([]byte, 100)
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			n, err := a.Serial.Read(buffer)
			a.handle(buffer[:n])
			if err != nil {
				log.L.Error("read from serial err: ", err)
			}
			time.Sleep(PassiveQueryInterval)
		}
	}
}

//handle data from serial
func (a *App) handle(data []byte) {
	a.cache.Write(data)

	//起始符 报警命令 控制器号 回路号 部位号 部件类型 时间年 时间月 时间日 时间时 时间分 时间秒 累加和 结束符
	d26 := [DataLength]byte{}
	for a.cache.Len() >= DataLength {
		n, err := a.cache.Read(d26[:])
		if n < DataLength {
			log.L.Error("get data's length is not 26")
			return
		}
		if err != nil {
			log.L.Error("read err: ", err)
			return
		}

		if data[0] != CodeStart {
			log.L.Error("the first byte is not 0x82")
			return
		}

		if data[DataLength-1] != CodeEnd {
			log.L.Error("the last byte is not 0x83")
			return
		}
		err = a.protocol.Decode(d26)
		if err != nil {
			log.L.Error("decode the data err: ", err)
			return
		}
		select {
		case a.chanProtocol <- a.protocol:
		default:
		}
	}
}

// Protocol get Protocol data, it will be block
func (a *App) Protocol() *Protocol {
	return <-a.chanProtocol
}
