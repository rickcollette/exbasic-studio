!include "MUI2.nsh"

!ifndef VERSION
  !define VERSION "1.0.0"
!endif
!ifndef PLATFORM
  !define PLATFORM "win64"
!endif

Name "ex800 ${VERSION}"
OutFile "ex800-${VERSION}-${PLATFORM}.exe"
InstallDir "$PROGRAMFILES\ex800"
InstallDirRegKey HKLM "Software\ex800" ""

!define MUI_ICON "data\ex800.ico"
!define MUI_UNICON "data\ex800.ico"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "pkg\doc\COPYING.txt"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "$INSTDIR\ex800.exe"
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Section "ex800" SecMain
  SetOutPath "$INSTDIR"
  File /r "pkg\*"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  CreateDirectory "$SMPROGRAMS\ex800"
  CreateShortCut "$SMPROGRAMS\ex800\ex800.lnk" "$INSTDIR\ex800.exe"
  CreateShortCut "$SMPROGRAMS\ex800\Uninstall.lnk" "$INSTDIR\uninstall.exe"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\ex800" \
    "DisplayName" "ex800 ${VERSION}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\ex800" \
    "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\ex800" \
    "DisplayVersion" "${VERSION}"
SectionEnd

Section "Uninstall"
  RMDir /r "$INSTDIR"
  RMDir /r "$SMPROGRAMS\ex800"
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\ex800"
  DeleteRegKey HKLM "Software\ex800"
SectionEnd
