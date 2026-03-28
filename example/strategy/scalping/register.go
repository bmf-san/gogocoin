package scalping

import strategy "github.com/bmf-san/gogocoin/pkg/strategy"

func init() {
	strategy.Register("scalping", func() strategy.Strategy { return NewDefault() })
}
