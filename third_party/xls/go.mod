// 本地 patch 副本：github.com/extrame/xls（原库已停滞且无 go.mod）。
// patch 内容：NumberCol.String 对 General 格式数字的正确输出（原库误判为日期）。
module github.com/extrame/xls

go 1.25.0

require (
	github.com/extrame/goyymmdd v0.0.0-20210114090516-7cc815f00d1a
	github.com/extrame/ole2 v0.0.0-20160812065207-d69429661ad7
)
