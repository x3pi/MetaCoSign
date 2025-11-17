#include "my_logger.h"
#include "mvm_linker.hpp"

void MyLogger::LogString(int f, char* str)
{
    GoLogString(f, str);
}

void MyLogger::LogBytes(int f, unsigned char* d, int s)
{
    GoLogBytes(f, d, s);
}
