# 함께 쓰는 소프트웨어

실행 파일에 들어가는 것:

| 이름 | 라이선스 | 쓰는 곳 |
|---|---|---|
| Go 표준 라이브러리 | BSD-3-Clause, © The Go Authors | 프로그램 전체 |
| [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys) v0.27.0 | BSD-3-Clause, © The Go Authors | 윈도우 API 호출 |
| [github.com/wailsapp/go-webview2](https://github.com/wailsapp/go-webview2) v1.0.23 | MIT, © 2020 John Chadwick 외 | WebView2 화면 |

빌드할 때만 쓰는 것 (실행 파일에는 아이콘·매니페스트·버전 정보만 들어가요):

| 이름 | 라이선스 |
|---|---|
| [github.com/tc-hib/winres](https://github.com/tc-hib/winres) v0.3.1 | 0BSD, © 2021 Thomas Combeléran |
| [github.com/nfnt/resize](https://github.com/nfnt/resize) | ISC, © 2012 Jan Schlicht |
| [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) v0.12.0 | BSD-3-Clause, © The Go Authors |

각 라이선스 전문은 해당 모듈 안의 LICENSE 파일에 있어요 (`go mod download -json <모듈>`로 위치 확인).

실행 중에는 사용자의 PC에 설치된 Microsoft Edge WebView2 런타임을 써요(이 프로그램에 들어 있지 않아요).
