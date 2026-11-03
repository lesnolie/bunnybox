; sing-box for Windows installer.
;
; The official daemon's InstallUpdate (third_party/sing-box,
; experimental/boxdd/update_windows.go) runs an update only if it is an NSIS
; installer whose ProductName is "sing-box", whose FileVersion is newer than
; the running core and whose Authenticode signer is the daemon's. It launches
; the installer as the service account in the user's session with
; "--updated --force-run"; this installer then updates in place silently.
;
; Install and update follow scripts/dev/manage.ps1 and the official desktop
; client's installer (SagerNet/sing-box-for-desktop build/installer.nsh,
; GPL-3.0): stop the official service, replace the files, then let the
; official "service install" register and start it again; on failure the
; previous files come back.
;
; makensis /DVERSION=1.14.5 /DSOURCE=build\app /DOUTFILE=out.exe installer\sing-box.nsi

!ifndef VERSION
  !error "VERSION is required"
!endif
!ifndef SOURCE
  !error "SOURCE is required"
!endif
!ifndef OUTFILE
  !define OUTFILE "sing-box-for-windows-${VERSION}-windows-amd64-setup.exe"
!endif
!ifndef VERSION4
  !define VERSION4 "0.0.0.0"
!endif

!define APP_NAME "sing-box for Windows"
!define LAYOUT_KEY "Software\SagerNet\sing-box"
!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\sing-box-for-windows"

Unicode true
ManifestDPIAware true
SetCompressor /SOLID lzma
RequestExecutionLevel admin
Name "${APP_NAME}"
OutFile "${OUTFILE}"
InstallDir "$PROGRAMFILES64\${APP_NAME}"
BrandingText "${APP_NAME} ${VERSION}"

VIProductVersion "${VERSION4}"
VIAddVersionKey /LANG=1033 "ProductName" "sing-box"
VIAddVersionKey /LANG=1033 "FileVersion" "${VERSION}"
VIAddVersionKey /LANG=1033 "ProductVersion" "${VERSION}"
VIAddVersionKey /LANG=1033 "FileDescription" "${APP_NAME} Setup"
VIAddVersionKey /LANG=1033 "LegalCopyright" "GPL-3.0"

!include MUI2.nsh
!include LogicLib.nsh
!include FileFunc.nsh
!include x64.nsh

Var DataDir
Var Updating
Var Existing
Var Daemon

!define MUI_ICON "..\assets\installer.ico"
!define MUI_UNICON "..\assets\installer.ico"
!define MUI_ABORTWARNING

!insertmacro MUI_PAGE_DIRECTORY
!define MUI_DIRECTORYPAGE_VARIABLE $DataDir
!define MUI_PAGE_HEADER_TEXT "$(dataTitle)"
!define MUI_PAGE_HEADER_SUBTEXT "$(dataSubtitle)"
!define MUI_DIRECTORYPAGE_TEXT_TOP "$(dataText)"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "English"

LangString dataTitle ${LANG_SIMPCHINESE} "核心数据目录"
LangString dataTitle ${LANG_ENGLISH} "Core data folder"
LangString dataSubtitle ${LANG_SIMPCHINESE} "官方 sing-box 服务的工作目录"
LangString dataSubtitle ${LANG_ENGLISH} "Working directory of the official sing-box service"
LangString dataText ${LANG_SIMPCHINESE} "官方服务会检查此目录及其上级目录的权限；所在磁盘根目录若允许普通用户写入，安装会被拒绝。"
LangString dataText ${LANG_ENGLISH} "The official service checks this folder and its parents; a drive whose root ordinary users can write to is refused."
LangString needWindows64 ${LANG_SIMPCHINESE} "需要 64 位 Windows。"
LangString needWindows64 ${LANG_ENGLISH} "64-bit Windows is required."
LangString noInstallation ${LANG_SIMPCHINESE} "没有找到已安装的 ${APP_NAME}，无法更新。"
LangString noInstallation ${LANG_ENGLISH} "No installed ${APP_NAME} to update."
LangString appRunning ${LANG_SIMPCHINESE} "${APP_NAME} 仍在运行，请退出后重试。"
LangString appRunning ${LANG_ENGLISH} "${APP_NAME} is still running; quit it and try again."
LangString registerFailed ${LANG_SIMPCHINESE} "官方服务注册失败（代码 $0），已恢复原来的文件。查看安装详情了解原因。"
LangString registerFailed ${LANG_ENGLISH} "The official service could not be registered (code $0); the previous files were restored. See the details."

Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_ICONSTOP "$(needWindows64)" /SD IDOK
    Abort
  ${EndIf}
  SetRegView 64
  SetShellVarContext all
  ${GetParameters} $0
  ClearErrors
  ${GetOptions} $0 "--updated" $1
  ${If} ${Errors}
    StrCpy $Updating 0
  ${Else}
    StrCpy $Updating 1
    SetSilent silent
  ${EndIf}
  ; An existing installation: this installer's own keys, then the
  ; development layout scripts/dev/manage.ps1 writes.
  StrCpy $Existing 0
  ReadRegStr $1 HKLM "${LAYOUT_KEY}" "InstallDirectory"
  ${If} $1 == ""
    ReadRegStr $1 HKLM "${LAYOUT_KEY}" "ProxyGlassDevInstallDirectory"
  ${EndIf}
  ${If} $1 != ""
  ${AndIf} ${FileExists} "$1\sing-box.exe"
    StrCpy $INSTDIR $1
    StrCpy $Existing 1
  ${EndIf}
  ReadRegStr $DataDir HKLM "${LAYOUT_KEY}" "DaemonDataDirectory"
  ${If} $DataDir == ""
    StrCpy $DataDir "$COMMONPROGRAMDATA\sing-box"
  ${EndIf}
  ${If} $Updating == 1
  ${AndIf} $Existing == 0
    Abort
  ${EndIf}
FunctionEnd

; waitForApp waits for the application in $INSTDIR to exit: it quits itself
; after asking the core to start this update.
Function waitForApp
  StrCpy $1 0
  ${DoWhile} $1 < 60
    ClearErrors
    FileOpen $2 "$INSTDIR\sing-box.exe" a
    ${IfNot} ${Errors}
      FileClose $2
      Return
    ${EndIf}
    Sleep 500
    IntOp $1 $1 + 1
  ${Loop}
  MessageBox MB_ICONSTOP "$(appRunning)" /SD IDOK
  Abort
FunctionEnd

!macro moveTree FROM TO
  ${If} ${FileExists} "${FROM}"
    Rename "${FROM}" "${TO}"
  ${EndIf}
!macroend

Section "Install"
  SetRegView 64
  SetShellVarContext all
  StrCpy $Daemon "$INSTDIR\resources\daemon\sing-box-daemon.exe"
  ${If} $Existing == 1
    Call waitForApp
    DetailPrint "Stopping the sing-box service..."
    nsExec::ExecToLog '"$Daemon" service stop'
    Pop $0
    RMDir /r "$INSTDIR\.previous"
    CreateDirectory "$INSTDIR\.previous"
    !insertmacro moveTree "$INSTDIR\sing-box.exe" "$INSTDIR\.previous\sing-box.exe"
    !insertmacro moveTree "$INSTDIR\mygo-sqlite3.dll" "$INSTDIR\.previous\mygo-sqlite3.dll"
    !insertmacro moveTree "$INSTDIR\resources" "$INSTDIR\.previous\resources"
  ${Else}
    CreateDirectory "$INSTDIR"
    ; Administrators and SYSTEM own the files, users only read and run them:
    ; the official service refuses a daemon others can replace.
    nsExec::ExecToLog 'icacls "$INSTDIR" /setowner *S-1-5-32-544'
    Pop $0
    nsExec::ExecToLog 'icacls "$INSTDIR" /inheritance:r /grant:r *S-1-5-18:(OI)(CI)F *S-1-5-32-544:(OI)(CI)F *S-1-5-11:(OI)(CI)RX'
    Pop $0
  ${EndIf}

  SetOutPath "$INSTDIR"
  File /r "${SOURCE}\*"

  DetailPrint "Registering the sing-box service..."
  nsExec::ExecToLog '"$Daemon" service install --working-directory "$DataDir"'
  Pop $0
  ${If} $0 != 0
    ${If} $Existing == 1
      Delete "$INSTDIR\sing-box.exe"
      Delete "$INSTDIR\mygo-sqlite3.dll"
      RMDir /r "$INSTDIR\resources"
      !insertmacro moveTree "$INSTDIR\.previous\sing-box.exe" "$INSTDIR\sing-box.exe"
      !insertmacro moveTree "$INSTDIR\.previous\mygo-sqlite3.dll" "$INSTDIR\mygo-sqlite3.dll"
      !insertmacro moveTree "$INSTDIR\.previous\resources" "$INSTDIR\resources"
      nsExec::ExecToLog '"$Daemon" service install --working-directory "$DataDir"'
      Pop $1
    ${EndIf}
    MessageBox MB_ICONSTOP "$(registerFailed)" /SD IDOK
    Abort
  ${EndIf}
  RMDir /r "$INSTDIR\.previous"

  ; The installation layout the application checks (internal/daemon).
  WriteRegDWORD HKLM "${LAYOUT_KEY}" "LayoutVersion" 2
  ReadRegStr $1 HKLM "${LAYOUT_KEY}" "InstallationID"
  ${If} $1 == ""
    System::Call 'ole32::CoCreateGuid(g .r1)'
    StrCpy $1 $1 36 1
    WriteRegStr HKLM "${LAYOUT_KEY}" "InstallationID" $1
  ${EndIf}
  WriteRegStr HKLM "${LAYOUT_KEY}" "DaemonDataDirectory" "$DataDir"
  WriteRegStr HKLM "${LAYOUT_KEY}" "InstallDirectory" "$INSTDIR"

  WriteUninstaller "$INSTDIR\uninstall.exe"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayName" "${APP_NAME}"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\sing-box.exe"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegDWORD HKLM "${UNINSTALL_KEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINSTALL_KEY}" "NoRepair" 1
  CreateShortcut "$SMPROGRAMS\${APP_NAME}.lnk" "$INSTDIR\sing-box.exe"
SectionEnd

Function un.onInit
  SetRegView 64
  SetShellVarContext all
FunctionEnd

Section "Uninstall"
  StrCpy $Daemon "$INSTDIR\resources\daemon\sing-box-daemon.exe"
  nsExec::ExecToLog '"$Daemon" service uninstall'
  Pop $0
  Delete "$INSTDIR\sing-box.exe"
  Delete "$INSTDIR\mygo-sqlite3.dll"
  RMDir /r "$INSTDIR\resources"
  RMDir /r "$INSTDIR\.previous"
  Delete "$INSTDIR\uninstall.exe"
  Delete "$INSTDIR\.proxyglass-dev-install.json"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\${APP_NAME}.lnk"
  ; The core's data folder and each user's profiles stay.
  DeleteRegKey HKLM "${UNINSTALL_KEY}"
  DeleteRegValue HKLM "${LAYOUT_KEY}" "InstallDirectory"
  DeleteRegValue HKLM "${LAYOUT_KEY}" "LayoutVersion"
  DeleteRegValue HKLM "${LAYOUT_KEY}" "InstallationID"
  DeleteRegValue HKLM "${LAYOUT_KEY}" "DaemonDataDirectory"
  ; Values and marker of a development installation this one replaced.
  DeleteRegValue HKLM "${LAYOUT_KEY}" "ProxyGlassDevOwner"
  DeleteRegValue HKLM "${LAYOUT_KEY}" "ProxyGlassDevInstallDirectory"
  DeleteRegKey /ifempty HKLM "${LAYOUT_KEY}"
SectionEnd
