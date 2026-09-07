[Setup]
AppId={{31452F0D-0DE2-4674-BE58-BB685EE5BA9F}
AppName=Relay Pilot
AppVersion=0.1.0
AppPublisher=Relay Pilot contributors
AppPublisherURL=https://github.com/InnoKesha1/relay-pilot
DefaultDirName={autopf}\Relay Pilot
DefaultGroupName=Relay Pilot
OutputDir=..\dist
OutputBaseFilename=RelayPilot-Windows-x64-Setup
SetupIconFile=runner\resources\app_icon.ico
UninstallDisplayIcon={app}\RelayPilot.exe
LicenseFile=..\LICENSE.md
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=admin
Compression=lzma2
SolidCompression=yes
CloseApplications=yes
RestartApplications=no

[Languages]
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "..\build\windows\x64\runner\Release\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "..\LICENSE.md"; DestDir: "{app}"
Source: "..\README.md"; DestDir: "{app}"

[Icons]
Name: "{group}\Relay Pilot"; Filename: "{app}\RelayPilot.exe"
Name: "{commondesktop}\Relay Pilot"; Filename: "{app}\RelayPilot.exe"

[Registry]
Root: HKCR; Subkey: "relaypilot"; ValueType: string; ValueData: "URL:Relay Pilot"; Flags: uninsdeletekey
Root: HKCR; Subkey: "relaypilot"; ValueName: "URL Protocol"; ValueType: string; ValueData: ""
Root: HKCR; Subkey: "relaypilot\shell\open\command"; ValueType: string; ValueData: """{app}\RelayPilot.exe"" ""%1"""
