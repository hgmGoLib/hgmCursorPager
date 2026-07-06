package hgmCursorPager

import (
	"encoding/json"
)

// Direction_t 游标方向枚举. 前端不感知, 仅后端使用.
type Direction_t string

const Direction_next Direction_t = "next"
const Direction_prev Direction_t = "prev"

// Token_t 不透明 token 内部结构.
// C 由调用方定义 (1-3 字段的 struct, 字段需可 JSON 序列化).
// 前端只持有 token 字符串原样回传, 永不 JSON.parse.
type Token_t[C any] struct {
	Direction Direction_t
	Cursor    C
}

// Encode 把 token JSON 序列化成字符串. 编码失败返回 "".
func Encode[C any](direction Direction_t, cursor C) string {
	tok := Token_t[C]{Direction: direction, Cursor: cursor}
	b, err := json.Marshal(&tok)
	if err != nil {
		return ""
	}
	return string(b)
}

// Decode 解码 token. 空 / JSON 失败 / Direction 非法 一律返回 hasToken=false.
// 调用方拿到 false 应按"第 1 页"处理.
func Decode[C any](s string) (tok Token_t[C], hasToken bool) {
	if s == "" {
		return Token_t[C]{}, false
	}
	if err := json.Unmarshal([]byte(s), &tok); err != nil {
		return Token_t[C]{}, false
	}
	if tok.Direction != Direction_next && tok.Direction != Direction_prev {
		return Token_t[C]{}, false
	}
	return tok, true
}

// BuildTokens 按协议规范计算 PrevToken / NextToken.
//
//	items:        本页条目 (按 cursor DESC 已排好, 不含合成项).
//	hasToken:     该次请求是否带了有效 token (false = 第 1 页).
//	reqDirection: 请求方向 (仅当 hasToken=true 时有意义).
//	pageSize:     该列表的 PageSize.
//	getCursor:    把 item 转成 cursor 值.
//
// 规则:
//   - len(items) == 0: 两个都返回 "".
//   - len(items) >= pageSize: NextToken 编码 last; 否则 NextToken="".
//   - !hasToken (第 1 页): PrevToken="".
//   - reqDirection==prev 且 len < pageSize (已到顶): PrevToken="".
//   - 其余: PrevToken 编码 first.
func BuildTokens[T any, C any](
	items []T,
	hasToken bool,
	reqDirection Direction_t,
	pageSize int,
	getCursor func(T) C,
) (prevToken, nextToken string) {
	if len(items) == 0 {
		return "", ""
	}
	if len(items) >= pageSize {
		last := items[len(items)-1]
		nextToken = Encode(Direction_next, getCursor(last))
	}
	switch {
	case !hasToken:
		prevToken = ""
	case reqDirection == Direction_prev && len(items) < pageSize:
		prevToken = ""
	default:
		first := items[0]
		prevToken = Encode(Direction_prev, getCursor(first))
	}
	return prevToken, nextToken
}
