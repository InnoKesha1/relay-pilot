# Сборка и версии

Исходная точка Hiddify App v4.1.1: `abbd671bf6bf05195acd4158c714bff267cada8f`. Подмодуль ядра: `c9d6f0f00b2eda34e4fb71863e4e0a62b3e931a0`. Приложение использует опубликованные библиотеки Hiddify Core **4.1.0**, Flutter **3.38.5** / Dart **3.10.4**. Серверы и изолированный стенд — sing-box **1.13.0**. Это разные компоненты; успешный серверный стенд не заменяет проверку нативного ядра на устройстве.

`pubspec.lock` и `service/go.sum` фиксируют граф зависимостей. Git-зависимости Flutter имеют конкретные commit refs. `tool/fetch_core.py` проверяет SHA-256 архивов до распаковки. Сторонние Actions закреплены по commit SHA. Версии перечислены в `tool/versions.json`.

## Локальные проверки

```sh
flutter pub get --enforce-lockfile
dart run build_runner build --delete-conflicting-outputs
flutter analyze --no-pub --no-fatal-infos --no-fatal-warnings lib
flutter test --no-pub test/relay
cd service
go test ./...
```

Для сетевого стенда установите sing-box 1.13.0, укажите полный путь в `SING_BOX_BIN`, запустите:

```sh
go test -tags=integration ./internal/pilot -run TestNetwork -v -count=1
```

Тест поднимает настоящие локальные Reality, Hysteria2 и VLESS/TLS процессы, собственный HTTPS/DNS endpoint и проверяет выходной IP. Он использует адреса loopback и прокси-вход вместо системного TUN. Конфигурации с лабораторными CA никогда не выдаются производственным API. Для Linux дополнительно: `python3 -m unittest discover -s ops/tests`.

## Запуск Actions

Исходники предназначены для ветки `pilot` открытого форка `InnoKesha1/relay-pilot`. Перед распространением клиентских сборок публикуйте актуальный исходный код форка и собирайте релизы через GitHub Actions согласно сохранённой лицензии. В новом форке GitHub может потребовать включить Actions на вкладке Actions. Android-сборка требует четыре секрета подписи, перечисленные ниже.

После публикации `.github/workflows/relay-pilot.yml` выполняет Go-тесты, сетевой стенд, Flutter-проверки, сборку APK и установщика Windows. Старые workflow перемещены в `docs/upstream-workflows/` как неисполняемые справочные файлы. Автоматической отправки в магазины нет. Actions выдаёт артефакты; публичный релиз оформляется после проверки на устройствах.

Для Android один раз создайте **свой постоянный** JKS вне репозитория (JDK `keytool -genkeypair`, RSA 3072+, срок 10 лет, отдельный alias). Сохраните резервную копию. Задайте секреты репозитория:

- `ANDROID_SIGNING_KEY` — base64 содержимого JKS;
- `ANDROID_SIGNING_STORE_PASSWORD` — пароль хранилища;
- `ANDROID_SIGNING_KEY_PASSWORD` — пароль ключа;
- `ANDROID_SIGNING_KEY_ALIAS` — alias.

Workflow останавливает Android-сборку, если ключ отсутствует. Постоянный ключ нужен для установки обновления поверх старой версии. Для Windows используется собственный Inno Setup AppId, каталог Relay Pilot, схема `relaypilot://`. Бинарник требует прав администратора для TUN. Подпись Authenticode в пилоте не настроена; установщик Windows будет неподписанным. Сборка APK и установщика здесь пока не выполнена.

Названия выходных файлов: `RelayPilot-Windows-x64-Setup.exe`, Android `app-release.apk`. APK applicationId: `org.relaypilot.client`. Обновляйте поверх установленной версии с тем же ключом/идентификатором, затем проверяйте сохранённый профиль и подключение. Номер сборки в Actions возрастает с `github.run_number`; не сбрасывайте его ниже уже установленного.

## QR для выдачи доступа

После установки зависимостей, находясь в корне проекта:

```sh
dart run tool/subscription_qr.dart /private/access.png < /private/access-url.txt
```

В PowerShell: `Get-Content -LiteralPath C:\private\access-url.txt | dart run tool/subscription_qr.dart C:\private\access.png`. Вход содержит только персональную HTTPS-ссылку. Файл и PNG — секреты; не добавляйте их в Git.

## Что нужно проверить перед выпуском

Установка Android/Windows, запрос VPN-разрешения, импорт камерой/из файла, raw-режим **встроенного** ядра, обновление поверх старой версии, DNS в захвате трафика на устройстве, смена сети, сон/пробуждение, отзыв активных сессий. Windows требует поддержку symlink для Flutter-плагинов и Visual Studio с Desktop development with C++; Android — JDK 17, SDK 36 и NDK 28.2.13676358.


## Проверка защищённого RPC Windows

После `python tool/fetch_core.py windows`, `pub get` и codegen:

```powershell
$env:RELAY_NATIVE_RPC_TEST='1'
$env:PATH="$PWD/hiddify-core/bin;$env:PATH"
flutter test --no-pub test/relay/native_rpc_test.dart
```

Этот тест использует настоящую DLL, но не включает системный VPN. Инициализирует временную базу, проверяет разрешённый mTLS-клиент, отклонение plaintext и TLS без клиентского сертификата, повторную инициализацию. В обычном прогоне тест помечен skipped; workflow Windows запускает его отдельно. Временная база ядра остаётся в системном temp до очистки — библиотека держит открытые файлы до завершения процесса.

Клиентский ключ RPC существует только в памяти Flutter. Сертификат сервера извлекается через FFI/method channel и закрепляется при TLS-подключении. В ядре 4.1.0 нет localhost SAN: проверка разрешает только тот же сертификат и действующий срок. Исходный RPC-сертификат имеет срок один год; перед его истечением понадобится обновление/пересоздание локального состояния ядра. Не включайте insecure-режим как обход ошибки сертификата.

## Транспортная настройка

Оба конечных VLESS outbound и VLESS inbound выхода используют [smux](https://sing-box.sagernet.org/configuration/shared/multiplex/). Он устраняет воспроизведённую в стенде потерю коротких HTTP-ответов на маршруте B при закрытии внешнего транспорта. Топология, detour и TLS остаются прежними. Регрессия входит в `TestNetworkChainsAndFailover`: по 100 `Connection: close` запросов на каждом маршруте, затем DNS, пять параллельных соединений и отказы. `RELAY_LAB_CONNECTION_CLOSE=1` завершает тест после коротких запросов для ускоренной диагностики.

ZIP с исходниками не содержит `.git`, SDK, кэши и исполняемые библиотеки. Для сборки приложения достаточно закреплённых зависимостей и `tool/fetch_core.py`; получать подмодуль отдельно для этого не требуется. В рабочем каталоге сохранён Git checkout тега v4.1.1 для дальнейшего оформления форка. Исходный код ядра закреплён gitlink и указан в `tool/versions.json`; его вложенные подмодули для пересборки самого ядра получают по исходным `.gitmodules`.
