package embed

import (
	_ "embed"
)

// StockBasicJSON 嵌入的股票基础数据
// 编译时从 data/stock_basic.json 嵌入到二进制文件中
//
//go:embed stock_basic.json
var StockBasicJSON []byte
