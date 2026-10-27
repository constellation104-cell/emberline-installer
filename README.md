# 잔불 서버 설치기

마인크래프트 **잔불 서버**에 들어가기 위한 설치 도우미예요. 서버는 CurseForge 모드팩 **Soulrend** 위에서 돌아가요.
CurseForge 앱으로 Soulrend를 설치해 두면, 이 프로그램이 그 프로필에 서버 접속에 필요한 것들을 더해 줘요.

> English: a small Windows helper that prepares a CurseForge "Soulrend" profile for the Emberline Minecraft server
> (adds the server's own mod, Korean language packs, the server entry and matching settings). MIT licensed.

이 저장소는 설치기 **프로그램의 소스 전체**예요. 실행 파일(.exe)이 이 소스에서 그대로 만들어졌다는 걸
누구나 확인할 수 있도록 공개해요.

## 하는 일

설치 버튼을 누르면, 고른 Soulrend 프로필 폴더 **안에서만** 아래 일을 해요. 바꾸기 전의 파일은
`.bak-installer`로 한 번 남겨 둬요.

1. 잔불 모드(`emberline-<버전>.jar`)를 `mods`에 넣어요. 예전 버전의 잔불 모드는 지워요.
2. 서버에 없는 모드는 CurseForge처럼 꺼요. 파일 이름 뒤에 `.disabled`를 붙이고, `minecraftinstance.json`에 꺼짐으로 표시해요.
   예전 설치기가 꺼 둔 모드 중 서버가 다시 쓰는 모드는 다시 켜요.
3. Xaero 미니맵 표시를 한 번 꺼요. 잔불 미니맵과 겹치기 때문이에요.
4. 서버에 있는데 모드팩에 없는 모드를 **Modrinth**(`cdn.modrinth.com`)와 **CurseForge 파일 서버**(`edge.forgecdn.net`)에서 받아요.
   - 받은 파일은 목록에 적힌 **SHA-512 값과 맞을 때만** 써요.
   - 다른 주소에서는 받지 않아요. `allowedURL` 함수를 보면 돼요.
5. 몇 가지 설정 파일을 서버와 맞춰요. Every Compat 블록 목록처럼 서버와 다르면 접속이 거부되는 항목들이에요.
   Sinytra Connector가 잘못 만든 변환 파일(`mods/.connector`)이 있으면 그 파일만 다시 만들도록 표시해요.
6. 플레이어가 **직접 받은** 모루(moru.gg) 한글 패치가 다운로드 폴더에 있으면 넣어요. 이 프로그램은 모루 패치를 받거나 갖고 있지 않아요.
7. 한글 보충 리소스팩을 넣고, 게임 언어를 한국어로, 리소스팩 순서를 맞춰요.
8. 멀티플레이 목록에 잔불 서버를 추가해요. 다른 서버 항목은 그대로 둬요.
9. 공식 서버(Realms) 안내 화면을 꺼요.
10. 고른 만큼 메모리를 CurseForge 프로필에 설정해요.

## 하지 않는 일

- **관리자 권한을 쓰지 않아요.** 매니페스트가 `asInvoker`예요.
- 레지스트리, 시작 프로그램, 예약 작업, 서비스, 방화벽, 백신 설정을 건드리지 않아요.
- 정보를 밖으로 보내지 않아요. 계정, 토큰, 브라우저 데이터에 손대지 않고, 사용 통계도 없어요.
- 위 4번의 모드 다운로드 말고는 인터넷을 쓰지 않아요.
  - CurseForge 페이지, 모루 페이지, WebView2 설치 페이지는 플레이어가 버튼을 누를 때만 기본 브라우저로 열어요.
- 다른 프로그램을 몰래 실행하지 않아요. 다음 두 경우만 있어요.
  - 「CurseForge 열기」를 누르면 CurseForge 앱을 켜요.
  - 폴더 고르기는 윈도우 기본 폴더 창을 직접 띄워요. 1.2.24까지 쓰던 PowerShell은 1.2.25에서 없앴어요.
- 찾는 범위가 정해져 있어요.
  - CurseForge 폴더가 기본 위치에 없을 때만 다른 드라이브의 폴더 이름을 훑어요. 파일 내용은 읽지 않아요.
  - 다운로드·바탕 화면 폴더에서는 모루 패치와 이 설치기의 zip만 찾아요.

화면은 Microsoft Edge **WebView2** 런타임으로 그려요. 대부분의 윈도우 10·11에 이미 들어 있어요.
작업 기록은 `%TEMP%\emberline-installer\log.txt`에, 화면용 임시 파일은 같은 폴더의 `webview`에 남아요.

## 데이터 팩 (`잔불데이터.pack`)

서버마다 다른 것들은 프로그램 안에 없고, 배포 zip 안의 `잔불데이터.pack`에 따로 들어 있어요.

- 서버 주소와 이름
- 잔불 모드 jar
- 한글 보충팩
- 모드 목록

데이터 팩에는 서버 주인의 **ed25519 서명**이 붙어 있어요.
프로그램은 `pack.go`의 공개 키와 서명이 맞는 데이터 팩만 써요. 누가 데이터 팩을 바꾸거나 새로 만들면 설치를 거부해요.
그래서 이 공개 프로그램을 가져다 다른 jar를 끼워 넣어 퍼뜨릴 수 없어요.

다른 서버에서 쓰려면 이렇게 해요.

```sh
go run ./tools/packtool keygen my.key          # 공개 키가 출력돼요 → pack.go의 packPublicKey에 넣기
go run ./tools/packtool build pack/ 잔불데이터.pack my.key
go run ./tools/packtool verify 잔불데이터.pack <공개 키>
```

`pack/` 폴더에 넣을 파일은 `pack.go`의 `packFiles`를 보면 돼요. 개인 키 파일은 절대 저장소에 올리지 마세요. `.gitignore`가 막아 줘요.

## 빌드하고 확인하기

Go 1.24.7이 필요해요. 어느 운영체제에서 빌드해도 결과가 같아요.

```sh
go test ./...
go run ./tools/winresgen assets/icon_256.png rsrc_windows_amd64.syso 1.3.0.0
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-H windowsgui -s -w -buildid=" -o EmberlineInstaller.exe .
sha256sum EmberlineInstaller.exe
```

GitHub Actions가 같은 명령으로 빌드해요(`.github/workflows/build.yml`). 결과의 SHA-256은 릴리스의 `SHA256SUMS.txt`에 올라가요.
같은 버전의 소스와 Go로 빌드하면 **똑같은 파일**이 나와요.
배포 zip 안의 `잔불서버설치.exe`는 이 파일과 이름만 달라요. 윈도우 PowerShell에서
`Get-FileHash 잔불서버설치.exe`로 값을 비교할 수 있어요.

## 백신이 경고할 때

이 프로그램은 아직 **코드 서명이 없어요.** 서명 없는 새 프로그램이라 이런 화면이 뜰 수 있어요.

- 파란 「Windows의 PC 보호」 화면
  - 바이러스 판정이 아니에요. 처음 보는 프로그램이라는 뜻이에요.
  - 「추가 정보 → 실행」을 누르면 실행돼요.
- 백신이 `…!ml`로 끝나는 이름으로 잡는 경우
  - 머신러닝 추측 판정이에요. Go 언어로 만든 서명 없는 프로그램이 자주 겪는 오탐이에요.
  - 위의 방법으로 파일이 이 소스와 같은지 확인할 수 있어요. 오탐은 Microsoft에 신고하고 있어요.

## 코드 서명 정책

무료 오픈소스 코드 서명(SignPath Foundation)을 신청할 예정이에요. 서명을 받으면 여기에 그 내용을 적어요.

- 서명되는 파일은 이 저장소의 GitHub Actions가 만든 것뿐이에요.
- 릴리스마다 관리자가 직접 승인해요.
- 역할
  - 작성·검토·승인: Invictus_1

**개인정보**: 이 프로그램은 사용자가 요청하지 않는 한 다른 시스템으로 어떤 정보도 보내지 않아요.
(This program will not transfer any information to other networked systems unless specifically requested by the user
or the person installing or operating it.)

## 라이선스

MIT, © 2026 Invictus_1 — [LICENSE](LICENSE). 함께 쓰는 라이브러리는 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)에 있어요.

이 프로그램은 Mojang, Microsoft, CurseForge(Overwolf), Modrinth, moru.gg, Soulrend 제작자와 관계가 없어요.
설치기가 받는 모드들은 각 제작자의 것이고, 각 사이트의 공식 배포 파일 그대로예요.
