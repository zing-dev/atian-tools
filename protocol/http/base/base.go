package base

type IHttp interface {
	Ping()
}

type ITemp interface {
	Temp(interface{}) interface{}
}
