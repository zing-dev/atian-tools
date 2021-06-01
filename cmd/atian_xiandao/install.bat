@echo off
rem run this script as admin

if not exist atian-xiandao.exe (
    echo atian-xiandao.exe not found
    goto :exit
)

sc create atian-xiandao binpath= "%CD%\atian-xiandao.exe" start= auto DisplayName= "atian-xiandao"
sc description atian-xiandao "atian-xiandao.exe"
sc start atian-xiandao
sc query atian-xiandao

:exit