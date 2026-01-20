/* Configuration for Opus codec */
#ifndef CONFIG_H
#define CONFIG_H

/* Use C99 variable-size arrays */
#define VAR_ARRAYS 1

/* Enable floating point */
#define FLOAT_APPROX 1

/* Define for runtime CPU detection */
#define OPUS_HAVE_RTCD 1

/* Disable assembly optimizations for portability */
/* #undef OPUS_ARM_ASM */
/* #undef OPUS_ARM_INLINE_ASM */
/* #undef OPUS_ARM_INLINE_EDSP */
/* #undef OPUS_ARM_INLINE_MEDIA */
/* #undef OPUS_ARM_INLINE_NEON */
/* #undef OPUS_ARM_MAY_HAVE_EDSP */
/* #undef OPUS_ARM_MAY_HAVE_MEDIA */
/* #undef OPUS_ARM_MAY_HAVE_NEON */
/* #undef OPUS_ARM_MAY_HAVE_NEON_INTR */
/* #undef OPUS_ARM_NEON_INTR */
/* #undef OPUS_X86_MAY_HAVE_SSE */
/* #undef OPUS_X86_MAY_HAVE_SSE2 */
/* #undef OPUS_X86_MAY_HAVE_SSE4_1 */
/* #undef OPUS_X86_MAY_HAVE_AVX */

/* Custom modes support */
/* #undef CUSTOM_MODES */

/* Do not build the floating-point API */
/* #undef DISABLE_FLOAT_API */

/* Assertions */
/* #undef ENABLE_ASSERTIONS */

/* Fuzzing */
/* #undef FUZZING */

/* Hardening */
/* #undef ENABLE_HARDENING */

/* Define to 1 if you have <alloca.h> */
#define HAVE_ALLOCA_H 1

/* Define to 1 if you have <dlfcn.h> */
#define HAVE_DLFCN_H 1

/* Define to 1 if you have <inttypes.h> */
#define HAVE_INTTYPES_H 1

/* Define to 1 if you have <memory.h> */
#define HAVE_MEMORY_H 1

/* Define to 1 if you have <stdint.h> */
#define HAVE_STDINT_H 1

/* Define to 1 if you have <stdlib.h> */
#define HAVE_STDLIB_H 1

/* Define to 1 if you have <strings.h> */
#define HAVE_STRINGS_H 1

/* Define to 1 if you have <string.h> */
#define HAVE_STRING_H 1

/* Define to 1 if you have <sys/stat.h> */
#define HAVE_SYS_STAT_H 1

/* Define to 1 if you have <sys/types.h> */
#define HAVE_SYS_TYPES_H 1

/* Define to 1 if you have <unistd.h> */
#define HAVE_UNISTD_H 1

/* Define to 1 if you have the `lrint' function. */
#define HAVE_LRINT 1

/* Define to 1 if you have the `lrintf' function. */
#define HAVE_LRINTF 1

/* Define to 1 if you have the `__malloc_hook' function. */
/* #undef HAVE___MALLOC_HOOK */

/* Package bugreport address */
#define PACKAGE_BUGREPORT "opus@xiph.org"

/* Package name */
#define PACKAGE_NAME "opus"

/* Package string */
#define PACKAGE_STRING "opus 1.5.2"

/* Package tarname */
#define PACKAGE_TARNAME "opus"

/* Package URL */
#define PACKAGE_URL ""

/* Package version */
#define PACKAGE_VERSION "1.5.2"

/* Define to 1 if all of the C90 standard headers exist */
#define STDC_HEADERS 1

/* Version number of package */
#define VERSION "1.5.2"

/* Define WORDS_BIGENDIAN to 1 if your processor stores words with the most
   significant byte first */
/* #undef WORDS_BIGENDIAN */

/* Restrict */
#define restrict __restrict

/* Use alloca */
#if defined(__GNUC__)
#define USE_ALLOCA 1
#endif

#endif /* CONFIG_H */
