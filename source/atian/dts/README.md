## 标签 Tag

```
layer=01;group=CWJZ3;column=01;relay=A1,2,3,4|B1,2,3,4;row=02;warehouse=常温静置库;
```

### 防区坐标 Coordinate

#### 防区所属仓库 Warehouse

**可用的标签名**

```
	TagWarehouse = "warehouse|Warehouse|库" //示例 warehouse:w01
```

- warehouse
- Warehouse
- 库

##### 示例

`warehouse:w01`

表示当前的防区的所属仓库为`w01

#### 防区所属组 Group

**可用的标签名**

```
	TagGroup = "group|Group|w|W|组" //示例 group:g001
```

- group
- Group
- w
- W
- 组

##### 示例

`group:g001`

表示当前的防区的所属组为`g001`

#### 防区所属行 Row

**可用的标签名**

```
	TagRow = "row|Row|x|X|行|" //示例 row:1
```

- row
- Row
- x
- X
- 行

##### 示例

`row:1`

表示当前的防区的所属行为`1`

#### 防区所属列 Column

**可用的标签名**

```
	TagColumn = "column|Column|y|Y|列" //示例 column:1
```

- column
- Column
- y
- Y
- 列

##### 示例

`column:1`

表示当前的防区的所属列为`1`

#### 防区所属层 Layer

**可用的标签名**

```
	TagLayer = "layer|Layer|z|Z|层" //示例 layer:1

```

- layer
- Layer
- z
- Z
- 层

##### 示例

`layer:1`

表示当前的防区的所属层为`1`

### 防区继电器路数 Relay

**可用标签**

```
	TagRelay = "relay|Relay|继电器" //示例 relay:A1,2,3,4|B1,2,3,4
```

- relay
- Relay
- 继电器

##### 示例

`relay:A1,2,3,4|B1,2,3,4`

表示当前的防区的对应A标签的继电器1，2，3，4和B标签的1，2，3，4 路数
> 继电器标签A，B必须与平台上的继电器标签一一对应且唯一