package agent

import (
	"strings"
	"testing"
)

func TestMTGOnlyCountsTelegramAndKeepsExactIntegers(t *testing.T) {
	input := `# HELP mtg_telegram_traffic Telegram bytes
mtg_telegram_traffic{dc="1",direction="from_client",telegram_ip="1.1.1.1"} 101
mtg_telegram_traffic{dc="2",direction="from_client",telegram_ip="2.2.2.2"} 9.007199254740993e+15
mtg_telegram_traffic{dc="1",direction="to_client",telegram_ip="1.1.1.1"} 202
mtg_domain_fronting_traffic{direction="to_client"} 999999
`
	v, e := ReadMTG(strings.NewReader(input))
	if e != nil || v.Up != 9007199254741094 || v.Down != 202 {
		t.Fatal(v, e)
	}
	for _, bad := range []string{`mtg_telegram_traffic{direction="from_client"} NaN`, `mtg_telegram_traffic{direction="from_client"} -1`, `mtg_telegram_traffic{direction="from_client"} 1.5`} {
		if _, e := ReadMTG(strings.NewReader(bad)); e == nil {
			t.Fatal("invalid byte counter")
		}
	}
}
