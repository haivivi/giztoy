/* soxr configuration - generated for Bazel build */

#ifndef SOXR_CONFIG_H
#define SOXR_CONFIG_H

/* Build options */
#define SOXR_LIB 1
#define HAVE_FENV_H 1
#define HAVE_STDBOOL_H 1
#define HAVE_STDINT_H 1
#define HAVE_LRINT 1
#define HAVE_SINGLE_PRECISION 1
#define HAVE_DOUBLE_PRECISION 1
#define HAVE_SIMD 0 /* Disable SIMD for portability */

/* CR (Constant Rate) resampler configurations */
#define WITH_CR32 1   /* 32-bit float precision */
#define WITH_CR64 1   /* 64-bit double precision */
#define WITH_CR32S 0  /* 32-bit float SIMD (disabled) */
#define WITH_CR64S 0  /* 64-bit double SIMD (disabled) */

/* VR (Variable Rate) resampler */
#define WITH_VR32 1

/* FFT configuration */
#define PFFFT_DOUBLE 0
#define AVCODEC_FOUND 0
#define WITH_PFFFT 0
#define WITH_AVFFT 0

/* HI-PREC configuration */
#define WITH_HI_PREC_CLOCK 1

/* Version info */
#define SOXR_VERSION "0.1.3"
#define SOXR_THIS_VERSION_MAJOR 0
#define SOXR_THIS_VERSION_MINOR 1
#define SOXR_THIS_VERSION_PATCH 3

/* Visibility */
#if defined(_WIN32)
#define SOXR_API __declspec(dllexport)
#else
#define SOXR_API __attribute__((visibility("default")))
#endif

#endif /* SOXR_CONFIG_H */
