/* Configuration for LAME MP3 encoder */
#ifndef LAME_CONFIG_H
#define LAME_CONFIG_H

/* Define if you have <inttypes.h> */
#define HAVE_INTTYPES_H 1

/* Define if you have <stdint.h> */
#define HAVE_STDINT_H 1

/* Define if you have <stdlib.h> */
#define HAVE_STDLIB_H 1

/* Define if you have <string.h> */
#define HAVE_STRING_H 1

/* Define if you have <memory.h> */
#define HAVE_MEMORY_H 1

/* Define if you have <unistd.h> */
#define HAVE_UNISTD_H 1

/* Define if you have <fcntl.h> */
#define HAVE_FCNTL_H 1

/* Define if you have <sys/time.h> */
#define HAVE_SYS_TIME_H 1

/* Define if you have the `socket' function. */
/* #undef HAVE_SOCKET */

/* Define if you have the `strtol' function. */
#define HAVE_STRTOL 1

/* Define if you have the `strchr' function. */
#define HAVE_STRCHR 1

/* Define if you have the `memcpy' function. */
#define HAVE_MEMCPY 1

/* Define to 1 if you have IEEE754 compatible floating point */
#define HAVE_IEEE754_FLOAT32 1

/* Build with internal mpglib decoding support */
#define HAVE_MPGLIB 1

/* Disable debugging */
#define NDEBUG 1

/* IEEE754 representation */
#define TAKEHIRO_IEEE754_HACK 1

/* Package name */
#define PACKAGE "lame"

/* Version */
#define VERSION "3.100"

/* Floating point type */
typedef float FLOAT;
typedef double FLOAT8;

/* IEEE 754 float type for LAME */
typedef float ieee754_float32_t;

/* Disable SIMD for portability with zig cc */
/* #undef HAVE_XMMINTRIN_H */

#endif /* LAME_CONFIG_H */
