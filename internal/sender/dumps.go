package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"lion/internal/model"
	"lion/internal/script"
)

type changeKind int

const (
	changeAdded changeKind = iota
	changeModified
	changeRemoved
)

// fieldChange 描述一个字段的变化（用于 headers / params / body 对比）
type fieldChange struct {
	key  string
	old  string
	new  string
	kind changeKind
}

// sortedStringKeys 返回排序后的 map key，保证输出稳定
func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// diffStringMap 计算两个 map 的新增、修改、删除项
func diffStringMap(old, new map[string]string) []fieldChange {
	keys := make(map[string]struct{}, len(old)+len(new))
	for k := range old {
		keys[k] = struct{}{}
	}
	for k := range new {
		keys[k] = struct{}{}
	}

	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)

	changes := make([]fieldChange, 0)
	for _, k := range ordered {
		oldVal, hasOld := old[k]
		newVal, hasNew := new[k]
		switch {
		case !hasOld && hasNew:
			changes = append(changes, fieldChange{key: k, new: newVal, kind: changeAdded})
		case hasOld && !hasNew:
			changes = append(changes, fieldChange{key: k, old: oldVal, kind: changeRemoved})
		case oldVal != newVal:
			changes = append(changes, fieldChange{key: k, old: oldVal, new: newVal, kind: changeModified})
		}
	}
	return changes
}

// formatJSONValue 将值格式化为 JSON 字符串，便于展示字符串、数字、对象等
func formatJSONValue(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

// equalBody 比较两个 body 是否相等（统一转 JSON 后比较）
func equalBody(a, b interface{}) bool {
	jA, _ := json.Marshal(a)
	jB, _ := json.Marshal(b)
	return bytes.Equal(jA, jB)
}

// printStringMapChanges 打印 map 字段的新增 / 修改 / 删除
func printStringMapChanges(label string, old, new map[string]string) {
	changes := diffStringMap(old, new)
	if len(changes) == 0 {
		return
	}

	fmt.Printf("     🔧 %s（%d 处变化）:\n", label, len(changes))
	for _, c := range changes {
		switch c.kind {
		case changeAdded:
			fmt.Printf("          + %s = %s\n", c.key, formatJSONValue(c.new))
		case changeModified:
			fmt.Printf("          ~ %s: %s → %s\n", c.key, formatJSONValue(c.old), formatJSONValue(c.new))
		case changeRemoved:
			fmt.Printf("          - %s（原值 %s）\n", c.key, formatJSONValue(c.old))
		}
	}
}

// printBodyChanges 打印 body 的顶层字段变化；非对象 body 直接展示前后值
func printBodyChanges(before, after interface{}) {
	if equalBody(before, after) {
		return
	}

	beforeMap, beforeOK := before.(map[string]interface{})
	afterMap, afterOK := after.(map[string]interface{})
	if !beforeOK || !afterOK {
		fmt.Printf("     🔧 Body:\n          %s → %s\n", formatJSONValue(before), formatJSONValue(after))
		return
	}

	keys := make(map[string]struct{}, len(beforeMap)+len(afterMap))
	for k := range beforeMap {
		keys[k] = struct{}{}
	}
	for k := range afterMap {
		keys[k] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)

	changes := make([]fieldChange, 0)
	for _, k := range ordered {
		oldVal, hasOld := beforeMap[k]
		newVal, hasNew := afterMap[k]
		switch {
		case !hasOld && hasNew:
			changes = append(changes, fieldChange{key: k, new: formatJSONValue(newVal), kind: changeAdded})
		case hasOld && !hasNew:
			changes = append(changes, fieldChange{key: k, old: formatJSONValue(oldVal), kind: changeRemoved})
		case !equalBody(oldVal, newVal):
			changes = append(changes, fieldChange{
				key:  k,
				old:  formatJSONValue(oldVal),
				new:  formatJSONValue(newVal),
				kind: changeModified,
			})
		}
	}
	if len(changes) == 0 {
		return
	}

	fmt.Printf("     🔧 Body（%d 处变化）:\n", len(changes))
	for _, c := range changes {
		switch c.kind {
		case changeAdded:
			fmt.Printf("          + %s = %s\n", c.key, c.new)
		case changeModified:
			fmt.Printf("          ~ %s: %s → %s\n", c.key, c.old, c.new)
		case changeRemoved:
			fmt.Printf("          - %s（原值 %s）\n", c.key, c.old)
		}
	}
}

// resultFields 返回脚本实际输出的字段名，便于判断脚本是否漏返回字段
func resultFields(r *model.ScriptResult) []string {
	fields := make([]string, 0, 6)
	if r.Headers != nil {
		fields = append(fields, "headers")
	}
	if r.Params != nil {
		fields = append(fields, "params")
	}
	if r.Body != nil {
		fields = append(fields, "body")
	}
	if len(r.Assertions) > 0 {
		fields = append(fields, "assertions")
	}
	if len(r.Extracted) > 0 {
		fields = append(fields, "extracted")
	}
	if r.Output != "" {
		fields = append(fields, "output")
	}
	return fields
}

// printResultDetails 打印脚本返回内容中与排查相关的部分
func printResultDetails(r *model.ScriptResult) {
	if r == nil {
		return
	}

	if fields := resultFields(r); len(fields) > 0 {
		fmt.Printf("     📤 返回字段: %s\n", strings.Join(fields, ", "))
	}
	if r.Output != "" {
		fmt.Printf("     📝 %s\n", r.Output)
	}
	for _, a := range r.Assertions {
		symbol := "✅"
		if !a.Pass {
			symbol = "❌"
		}
		if a.Name != "" {
			fmt.Printf("     %s %s: %s\n", symbol, a.Name, a.Message)
		} else {
			fmt.Printf("     %s %s\n", symbol, a.Message)
		}
	}
	if len(r.Extracted) > 0 {
		fmt.Printf("     📦 extracted:\n")
		for _, k := range sortedStringKeys(r.Extracted) {
			fmt.Printf("          %s = %s\n", k, formatJSONValue(r.Extracted[k]))
		}
	}
}

// printScriptStepLine 打印单个脚本的执行状态
func printScriptStepLine(index int, path string, err error, elapsedMs int64) {
	symbol := "✅"
	if err != nil {
		symbol = "❌"
	}
	fmt.Printf("  %d. %s %s（%dms）\n", index+1, symbol, path, elapsedMs)
}

// printScriptStderr 打印脚本 stderr。
// 很多脚本会把诊断日志写到 stderr 但正常退出，需要在 --verbose 下展示，
// 否则会出现“脚本返回 headers 但什么都没改”却看不到原因的情况。
func printScriptStderr(stderr string) {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return
	}

	fmt.Printf("     🪵 stderr:\n")
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSuffix(line, "\r")
		fmt.Printf("          %s\n", line)
	}
}

// printPreScriptSteps 打印前置脚本执行详情（仅 --verbose）。
// headerLabel 用于区分 HTTP 的 Headers 与 gRPC 的 Metadata。
func printPreScriptSteps(steps []script.PreScriptStep, totalMs int64, headerLabel, apiID string) {
	if len(steps) == 0 {
		return
	}

	fmt.Printf("── 前置脚本（pre_request，%d 个脚本，共 %dms） ──────────────\n", len(steps), totalMs)
	if apiID != "" {
		fmt.Printf("   接口: %s\n", apiID)
	}
	for i, st := range steps {
		printScriptStepLine(i, st.Script, st.Error, st.ElapsedMs)
		if st.Error != nil {
			fmt.Printf("     ⚠️  %v\n", st.Error)
			fmt.Printf("     ⚠️  链式执行已中止，后续前置脚本不会再执行\n")
			continue
		}

		printScriptStderr(st.Stderr)
		printResultDetails(st.Result)
		if st.Before == nil || st.After == nil {
			continue
		}

		headersChanged := len(diffStringMap(st.Before.Headers, st.After.Headers)) > 0
		paramsChanged := len(diffStringMap(st.Before.Params, st.After.Params)) > 0
		bodyChanged := !equalBody(st.Before.Body, st.After.Body)

		if !headersChanged && !paramsChanged && !bodyChanged {
			fmt.Printf("     ℹ️  未修改 %s / params / body\n", headerLabel)
			continue
		}

		printStringMapChanges(headerLabel, st.Before.Headers, st.After.Headers)
		printStringMapChanges("Params", st.Before.Params, st.After.Params)
		printBodyChanges(st.Before.Body, st.After.Body)
	}
	fmt.Printf("──────────────────────────────────────────────\n\n")
}

// printPostScriptSteps 打印后置脚本执行详情（仅 --verbose）
func printPostScriptSteps(steps []script.PostScriptStep, apiID string) {
	if len(steps) == 0 {
		return
	}

	fmt.Printf("── 后置脚本（post_request，%d 个脚本） ──────────────\n", len(steps))
	if apiID != "" {
		fmt.Printf("   接口: %s\n", apiID)
	}
	for i, st := range steps {
		printScriptStepLine(i, st.Script, st.Error, st.ElapsedMs)
		if st.Error != nil {
			fmt.Printf("     ⚠️  %v\n", st.Error)
			fmt.Printf("     ⚠️  链式执行已中止，后续后置脚本不会再执行\n")
			continue
		}

		printScriptStderr(st.Stderr)
		printResultDetails(st.Result)
		if st.Result != nil && (st.Result.Headers != nil || st.Result.Params != nil || st.Result.Body != nil) {
			fmt.Printf("     ℹ️  headers/params/body 仅展示脚本返回值，后置脚本不会修改已发送的请求\n")
		}
	}
	fmt.Printf("──────────────────────────────────────────────\n\n")
}

// preChainError 返回前置脚本链中第一个错误
func preChainError(steps []script.PreScriptStep) error {
	for _, st := range steps {
		if st.Error != nil {
			return st.Error
		}
	}
	return nil
}

// postChainError 返回后置脚本链中第一个错误
func postChainError(steps []script.PostScriptStep) error {
	for _, st := range steps {
		if st.Error != nil {
			return st.Error
		}
	}
	return nil
}

// printStringMap 打印单值 map（headers / metadata）
func printStringMap(label string, m map[string]string) {
	fmt.Printf("%s（%d）:\n", label, len(m))
	for _, k := range sortedStringKeys(m) {
		fmt.Printf("  %s: %s\n", k, m[k])
	}
}

// printMultiMap 打印多值 map（http.Header / gRPC metadata.MD）
func printMultiMap(label string, m map[string][]string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Printf("%s（%d）:\n", label, len(m))
	for _, k := range keys {
		fmt.Printf("  %s: %s\n", k, strings.Join(m[k], ", "))
	}
}

// printBody 格式化输出请求体
func printBody(label string, body interface{}) {
	if body == nil {
		return
	}
	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		fmt.Printf("%s: %v\n", label, body)
		return
	}
	fmt.Printf("%s:\n%s\n", label, string(data))
}

// printHTTPRequest 打印 HTTP 请求（脚本执行前，或没有脚本时）
func printHTTPRequest(title, method, url string, headers map[string]string, body interface{}) {
	fmt.Printf("── %s ──────────────────────────\n", title)
	fmt.Printf("%s %s\n", method, url)
	printStringMap("Headers", headers)
	printBody("Body", body)
	fmt.Printf("──────────────────────────────────\n\n")
}

// printHTTPFinalRequest 打印真正发送的 HTTP 请求（脚本执行后）
func printHTTPFinalRequest(title string, req *http.Request, body interface{}) {
	fmt.Printf("── %s ──────────────────────────\n", title)
	fmt.Printf("%s %s\n", req.Method, req.URL.String())
	printMultiMap("Headers", req.Header)
	printBody("Body", body)
	fmt.Printf("──────────────────────────────────\n\n")
}

// printGRPCRequest 打印 gRPC 请求（脚本执行前，或没有脚本时）
func printGRPCRequest(title, host, path string, metadata map[string]string, body interface{}) {
	fmt.Printf("── %s ──────────────────────────\n", title)
	fmt.Printf("gRPC %s\n", path)
	fmt.Printf("Host: %s\n", host)
	printStringMap("Metadata", metadata)
	printBody("Body", body)
	fmt.Printf("──────────────────────────────────\n\n")
}

// printGRPCFinalRequest 打印真正发送的 gRPC 请求（脚本执行后）
func printGRPCFinalRequest(title, host, path string, metadata map[string][]string, body interface{}) {
	fmt.Printf("── %s ──────────────────────────\n", title)
	fmt.Printf("gRPC %s\n", path)
	fmt.Printf("Host: %s\n", host)
	printMultiMap("Metadata", metadata)
	printBody("Body", body)
	fmt.Printf("──────────────────────────────────\n\n")
}
