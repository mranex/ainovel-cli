package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/voocel/ainovel-cli/internal/host"
	"github.com/voocel/ainovel-cli/internal/host/trans"
)

// parseTranslateArgs phân tích `/translate [ch | N-M] [from=N] [to=M] [--force]`.
func parseTranslateArgs(args []string) (trans.Options, error) {
	var opts trans.Options
	for _, raw := range args {
		a := strings.TrimSpace(raw)
		if a == "" {
			continue
		}
		if a == "--force" || a == "-f" {
			opts.Force = true
			continue
		}
		if k, v, ok := strings.Cut(a, "="); ok {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil || n <= 0 {
				return trans.Options{}, fmt.Errorf("%s phải là số nguyên dương: %q", k, v)
			}
			switch strings.ToLower(strings.TrimSpace(k)) {
			case "from":
				opts.From = n
			case "to":
				opts.To = n
			default:
				return trans.Options{}, fmt.Errorf("tham số không hợp lệ %q (hỗ trợ: from / to / --force)", k)
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			return trans.Options{}, fmt.Errorf("cờ không hợp lệ %q", a)
		}
		if lhs, rhs, ok := strings.Cut(a, "-"); ok {
			from, err1 := strconv.Atoi(strings.TrimSpace(lhs))
			to, err2 := strconv.Atoi(strings.TrimSpace(rhs))
			if err1 != nil || err2 != nil || from <= 0 || to <= 0 || from > to {
				return trans.Options{}, fmt.Errorf("khoảng chương không hợp lệ: %q (ví dụ 1-10)", a)
			}
			opts.From = from
			opts.To = to
			continue
		}
		ch, err := strconv.Atoi(a)
		if err != nil || ch <= 0 {
			return trans.Options{}, fmt.Errorf("số chương không hợp lệ: %q (cú pháp: /translate [ch | N-M] [--force])", a)
		}
		opts.From = ch
		opts.To = ch
	}
	if opts.From > 0 && opts.To > 0 && opts.From > opts.To {
		return trans.Options{}, fmt.Errorf("khoảng chương không hợp lệ: from=%d > to=%d", opts.From, opts.To)
	}
	return opts, nil
}

func startTranslate(rt *host.Host, args []string) (trans.Options, error) {
	if rt == nil {
		return trans.Options{}, fmt.Errorf("chưa có bộ truyện đang mở")
	}
	opts, err := parseTranslateArgs(args)
	if err != nil {
		return trans.Options{}, err
	}
	if err := rt.StartTranslate(context.Background(), opts); err != nil {
		return trans.Options{}, err
	}
	return opts, nil
}
