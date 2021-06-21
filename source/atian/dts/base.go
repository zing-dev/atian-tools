package dts

const (
	FieldAt   = "at"
	FieldZHAt = "时间"

	FieldTemperature   = "temperature"
	FieldZHTemperature = "温度"

	FieldHost   = "host"
	FieldZHHost = "主机"

	FieldType   = "type"
	FieldZHType = "类型"

	FieldZoneName   = "zone_name"
	FieldZHZoneName = "防区名"

	FieldLocation   = "location"
	FieldZHLocation = "位置"
)

var (
	ZHHandleType = map[string]string{
		"alarm": "设备报警",
		"event": "光纤状态",
	}

	ZHAlarmField = map[string]string{
		FieldAt:          FieldZHAt,
		FieldZoneName:    FieldZHZoneName,
		FieldTemperature: FieldZHTemperature,
		FieldType:        FieldZHType,
		FieldLocation:    FieldZHLocation,
		FieldHost:        FieldZHHost,
	}
)
