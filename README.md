all-build
=========
Build your (C++) projects using all the machines

Detecting dependencies
----------------------
Many projects detect dependencies by pre-parsing the source code and letting the compiler to do a detection of the include files and other dependencies. Even though it is not a bad idea and it allows to get all the dependencies, it requires quite a lot of computation power to do it (not as much as the compilation itself). This project detects dependencies from the command line, so you need to specify all inputs/outputs explicitly.

This means that if you have any library in a system space that is in your client machine, but not in the execcutor machine (docker image), you can not simply specify -l<library> and expect it to work. You need to specify the path to the library explicitly, so this compiler can copy it to the executor machine.

This also means that if you specify include directory that is really, really large, it will take a lot of time to copy it to the executor machine. So, it is better to specify only the directories that are really needed. Consider the case when you specify "-I/usr/include". In this case, all few thousand files will be copied to the executor machine, even though you might need only a few of them.

Limitations
-----------
- **Works only on a nice codebase**: If you include `#include "../header.h"` parent directory, you should consider reworking your codebase, or specify the include directory explicitly by passing compiler parameter `-I`. This compiler does not support including parent directories, because it transfer only the context of the current file (and subdirectories).

What works
----------
I am using `conan` package manager and `CMake` with `ninja-build` and it works for my projects.

How to get the best performance
-------------------------------
- **Limit the context**: Separate logical parts of your codebase into separate directories. This will allow to limit the context of the compiler and reduce the amount of files that are copied to the executor machine.
- **Unlimited parallel compilations**: `CMake` and other build systems limit the number of concurrently compiled files to the number of your cores. Since this compiler is not CPU bound, you can set the number of parallel compilations to a very high number. For example, I have 8 cores, but I set the number of parallel compilations to 100. This allows to compile files in parallel and reduce the overall compilation time. My 8 cores have plenty of power to wait for the results of 100s of jobs :-) With `CMake` you  want to run your build with something like `cmake --build -j100` (or to set `"cmake.buildArgs"` to `"-j100"` in `settings.json` if you use vscode with `ms-vscode.cmake-tools` plugin).

# Contributing
This project tries to follow the standardized [directory structure](https://github.com/golang-standards/project-layout) and effective [Go patterns](https://go.dev/doc/effective_go).
