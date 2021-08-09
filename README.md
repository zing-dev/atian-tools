# ATian Tools 亚天软件开发工具库

## Protocol 自定义协议

> 注意: 各个项目一定根据实际情况用不同的协议,做好开发文档注释!!!

### HTTP HTTP协议

#### NanDu 南都项目专用

#### xiandao 先导项目专用

### Soap webservice项目用

### Q5

## Data Source 数据源

### ATian 所属亚天设备

#### DTS 分布式光纤测温

> 接口详见`source/atian/dts`目录下的源码

- 设备主机ID
- 设备防区
- 设备防区报警
- 设备通道温度信号数据
- 设备多有防区温度
- 设备光纤状态

#### DFVS 振动(暂未提供)

### BeiDa Bluebird 北大青鸟消防设备

> 串口开发,主要提供烟感温感报警,具体详见`source/beida_bluebird`源码

这四个参数确定一个烟雾报警设备的位置

- 控制器号
- 回路号
- 部位号
- 部件类型

### 必须的防区与设备的映射文件

```
第一行必须是防区名,防区编码,控制器号,回路号,部位号,部件类型
第一列必须是防区名
第二列必须是防区名唯一编码
第三列必须是控制器号,必须和设备上的控制器号对应
第四列必须是回路号,必须和设备上的回路号对应范围
第五列必须是部位号,必须和设备上的部位号对应范围
第六列必须是部件类型,必须和设备上部件类型的对应范围
```

#### 文档

- 旧版文档JBF193K
- 新版文档JBF293K

#### 报警命令

- `128 即 (0x80)`
- `0`

#### 部件类型

- 烟感 `21`