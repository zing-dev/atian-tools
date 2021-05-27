package device

const (
	ColorDefault = "default"
	ColorPrimary = "primary"
	ColorSuccess = "success"
	ColorInfo    = "info"
	ColorWarning = "warning"
	ColorDanger  = "danger"
)

const (
	_ Type = iota
	TypeDTS
	TypeRelay
	TypeApi
)

const (
	_ EventType = iota
	EventError
	EventAdd
	EventRun
	EventUpdate
	EventClose
	EventDelete
)
const (
	_ StatusType = iota
	Connecting
	Connected
	Disconnect
)

func (s *Type) String() string {
	switch *s {
	case TypeDTS:
		return "DTS"
	case TypeRelay:
		return "继电器"
	case TypeApi:
		return "API"
	default:
		return "未知设备"
	}
}

func (s *StatusType) String() string {
	switch *s {
	case Connecting:
		return "连接中"
	case Connected:
		return "已连接"
	case Disconnect:
		return "已断开"
	default:
		return "未知状态"
	}
}

func GetConnectMap() []Constant {
	a1, a2, a3 := Connecting, Connected, Disconnect
	return []Constant{
		{Name: a1.String(), Value: byte(Connecting), Color: ColorPrimary},
		{Name: a2.String(), Value: byte(Connected), Color: ColorDanger},
		{Name: a3.String(), Value: byte(Disconnect), Color: ColorDanger},
	}
}

func GetDeviceMap() []Constant {
	a1, a2, a3 := TypeDTS, TypeRelay, TypeApi
	return []Constant{
		{Name: a1.String(), Value: byte(TypeDTS), Color: ColorPrimary},
		{Name: a2.String(), Value: byte(TypeRelay), Color: ColorPrimary},
		{Name: a3.String(), Value: byte(TypeApi), Color: ColorPrimary},
	}
}

type (
	Type       byte
	EventType  byte
	StatusType byte

	Constant struct {
		Name  string `json:"name"`
		Value byte   `json:"value"`
		Color string `json:"color,omitempty"`
	}
)
