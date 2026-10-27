package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

//go:embed web/index.html
var indexHTML string

// UI relays {id, method, params} from the page to Go and evaluates the answer back.
type UI struct {
	env  *Env
	eval func(js string)
	quit func()
}

type rpcMsg struct {
	ID     int               `json:"id"`
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
}

func jsJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	s := strings.ReplaceAll(string(b), " ", ` `)
	return strings.ReplaceAll(s, " ", ` `)
}

func (u *UI) handle(raw string) {
	var m rpcMsg
	if err := json.Unmarshal([]byte(raw), &m); err != nil || m.Method == "" {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logf("panic in %s: %v", m.Method, r)
				u.eval(fmt.Sprintf("window.__ip&&__ip.done(%d,false,%s)", m.ID, jsJSON("내부 오류가 났어요")))
			}
		}()
		res, err := u.call(m.Method, m.Params)
		if err != nil {
			u.eval(fmt.Sprintf("window.__ip&&__ip.done(%d,false,%s)", m.ID, jsJSON(err.Error())))
			return
		}
		u.eval(fmt.Sprintf("window.__ip&&__ip.done(%d,true,%s)", m.ID, jsJSON(res)))
	}()
}

func param[T any](ps []json.RawMessage, i int) T {
	var v T
	if i < len(ps) {
		_ = json.Unmarshal(ps[i], &v)
	}
	return v
}

func (u *UI) call(method string, ps []json.RawMessage) (any, error) {
	e := u.env
	switch method {
	case "state":
		return e.state(), nil
	case "openInstall":
		openURL(installURL)
		return true, nil
	case "openPackPage":
		openURL(packPageURL)
		return true, nil
	case "openMoru":
		openURL(moruURL)
		return true, nil
	case "openFolder":
		p := param[string](ps, 0)
		if p != "" {
			openPath(p)
		}
		return true, nil
	case "pickFolder":
		p := pickFolder()
		if p == "" {
			return map[string]any{"picked": false}, nil
		}
		n := e.addRoot(p)
		return map[string]any{"picked": true, "found": n, "path": p}, nil
	case "openCurseForge":
		return openCurseForge(), nil
	case "install":
		dir := param[string](ps, 0)
		opts := param[InstallOptions](ps, 1)
		for _, in := range findInstances(e.rootsNow()) {
			if in.Dir == dir {
				if !in.Right {
					return nil, errors.New("Soulrend " + packVersion + "가 아니에요. CurseForge에서 버전을 바꿔 주세요")
				}
				if curseforgeRunning() {
					return nil, errors.New("CurseForge 앱이 켜져 있어요. 앱을 완전히 닫고 다시 눌러 주세요 (작업 표시줄 오른쪽 아래 아이콘도). 켜져 있으면 CurseForge가 모드 목록과 메모리를 다시 덮어써요")
				}
				progressHook = func(done, total int64, name string) {
					u.eval(fmt.Sprintf("window.__progress&&__progress(%d,%d,%s)", done, total, jsJSON(name)))
				}
				steps := install(in, findMoru(e.downloads), opts)
				progressHook = nil
				for _, s := range steps {
					logf("%v %s %s", s.OK, s.Title, s.Note)
				}
				return steps, nil
			}
		}
		return nil, errors.New("선택한 Soulrend 프로필을 찾지 못했어요")
	case "quit":
		u.quit()
		return true, nil
	}
	return nil, errors.New("알 수 없는 요청: " + method)
}
