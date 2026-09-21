; Connective Windows installer (Inno Setup 6).
; Built on Windows CI from the release tree; never hand-edited binaries.
;
;   iscc /DAppVersion=0.3.0-beta.1 /DSourceDir=C:\build\stage packaging\windows\connective.iss
;
; Layout (see docs/WINDOWS.md "Installed layout"):
;   {app}\versions\<ver>\...   immutable per-version trees
;   {app}\current.txt           active version pointer (atomically replaced)
;   {app}\updater\...           connective-updater.exe
; User data (%LOCALAPPDATA%\Connective) lives outside {app} and is
; never touched by install/upgrade; uninstall removes it only when the
; user ticks the task.
#define MyAppName "Connective"
#ifndef AppVersion
  #define AppVersion "0.0.0-dev"
#endif
#ifndef SourceDir
  #define SourceDir "stage"
#endif

[Setup]
AppId={{8E2C9F4A-3B1D-4E6A-9C2F-7A5B1D3E8F01}
AppName={#MyAppName}
AppVersion={#AppVersion}
AppVerName={#MyAppName} {#AppVersion}
AppPublisher=Connective
AppPublisherURL=https://github.com/calledLifelss/Connective
AppSupportURL=https://github.com/calledLifelss/Connective/issues
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
PrivilegesRequired=admin
PrivilegesRequiredOverridesAllowed=commandline
OutputDir=installer-out
OutputBaseFilename=Connective-{#AppVersion}-windows-x64-setup
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\versions\{#AppVersion}\connective.exe
; Upgrades reuse the same AppId: install-over works, shortcuts stay valid.
UsePreviousAppDir=yes
DirExistsWarning=no
; Restart Manager closes processes holding files being replaced (our
; image names only); the pre-install warning below sets expectations.
CloseApplications=yes
CloseApplicationsFilter=connective.exe,connectived.exe,connective-helper.exe,sing-box.exe

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "removedata"; Description: "Remove user data (%LOCALAPPDATA%\Connective: subscriptions, servers, settings)"; GroupDescription: "Uninstall options:"; Flags: unchecked

[Files]
; Versioned tree. Everything under versions\<ver> is immutable at runtime;
; the updater activates new versions by rewriting current.txt atomically.
Source: "{#SourceDir}\versions\{#AppVersion}\*"; DestDir: "{app}\versions\{#AppVersion}"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "{#SourceDir}\updater\*"; DestDir: "{app}\updater"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "{#SourceDir}\current.txt"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\versions\{#AppVersion}\connective.exe"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\versions\{#AppVersion}\connective.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\versions\{#AppVersion}\connective.exe"; Description: "{cm:LaunchProgram,{#MyAppName}}"; Flags: nowait postinstall skipifsilent shellexec

[Code]
const
  DataDirName = 'Connective';

{ Warn before replacing a live install: killing the core mid-tunnel
  drops the VPN. The user disconnects first, or accepts the drop. }
function InitializeSetup(): Boolean;
var
  Res: Integer;
begin
  Result := True;
  if MsgBox('Installing will stop any running Connective (VPN disconnects). Continue?',
    mbConfirmation, MB_OKCANCEL) = IDCANCEL then
    Result := False;
end;

{ Optional user-data wipe on uninstall. Default: preserved. }
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  DataDir: String;
begin
  if (CurUninstallStep = usPostUninstall) and WizardIsTaskSelected('removedata') then
  begin
    DataDir := ExpandConstant('{localappdata}\') + DataDirName;
    if DirExists(DataDir) then
      DelTree(DataDir, True, True, True);
  end;
end;
