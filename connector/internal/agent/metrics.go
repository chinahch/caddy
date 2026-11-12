package agent

import (
	"bufio"
	"errors"
	"io"
	"math/big"
	"regexp"
	"strings"
)

type Counters struct{ Up, Down int64 }

var metricLine = regexp.MustCompile(`^mtg_telegram_traffic(?:_total)?\{([^}]*)\}\s+(\S+)(?:\s+\d+)?$`)
var directionLabel = regexp.MustCompile(`(?:^|,)direction="(from_client|to_client)"(?:,|$)`)

func ReadMTG(r io.Reader) (Counters, error) {
	result := Counters{}
	scanner := bufio.NewScanner(io.LimitReader(r, 1<<20))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "mtg_telegram_traffic") {
			continue
		}
		parts := metricLine.FindStringSubmatch(line)
		if len(parts) == 0 {
			return result, errors.New("invalid MTG metric")
		}
		direction := directionLabel.FindStringSubmatch(parts[1])
		if len(direction) == 0 {
			return result, errors.New("invalid MTG metric direction")
		}
		v := parts[2]
		if len(v) > 64 {
			return result, errors.New("invalid byte counter")
		}
		f, _, e := big.ParseFloat(v, 10, 256, big.ToNearestEven)
		if e != nil || f.Sign() < 0 || f.IsInf() || f.Cmp(new(big.Float).SetInt64(1<<62)) > 0 {
			return result, errors.New("invalid byte counter")
		}
		n, accuracy := f.Int64()
		if accuracy != big.Exact {
			return result, errors.New("fractional byte counter")
		}
		if direction[1] == "from_client" {
			if n > (1<<62)-result.Up {
				return result, errors.New("byte counter overflow")
			}
			result.Up += n
		} else {
			if n > (1<<62)-result.Down {
				return result, errors.New("byte counter overflow")
			}
			result.Down += n
		}
	}
	return result, scanner.Err()
}
