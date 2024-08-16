@echo off
setlocal enabledelayedexpansion

set /p num_instances="Enter the number of instances to run: "

for /L %%i in (1,1,%num_instances%) do (
    start "" cmd /c post.cmd
)

echo Started %num_instances% instances of test.cmd