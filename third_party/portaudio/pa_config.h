#ifndef PA_CONFIG_H
#define PA_CONFIG_H

/* Standard headers */
#define HAVE_STDINT_H 1
#define HAVE_INTTYPES_H 1
#define HAVE_SYS_TYPES_H 1
#define HAVE_UNISTD_H 1
#define HAVE_STDLIB_H 1
#define HAVE_STRING_H 1

/* Clock support */
#define HAVE_NANOSLEEP 1

#ifdef __APPLE__
/* macOS CoreAudio support */
#define PA_USE_COREAUDIO 1
#define HAVE_CLOCK_GETTIME 0
#define PA_MAC_CORE_HAVE_COMPONENT_MANAGER 0
#elif defined(__linux__)
/* Linux ALSA support */
#define PA_USE_ALSA 1
#define HAVE_CLOCK_GETTIME 1
#endif

#endif /* PA_CONFIG_H */
