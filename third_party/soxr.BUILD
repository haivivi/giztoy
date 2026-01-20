# soxr source files
# soxr uses template-style includes, so we only compile the main files
filegroup(
    name = "soxr_c_srcs",
    srcs = [
        "src/soxr.c",
        "src/data-io.c",
        "src/dbesi0.c",
        "src/filter.c",
        "src/fft4g64.c",
        "src/cr64.c",
    ],
    visibility = ["//visibility:public"],
)

filegroup(
    name = "soxr_headers",
    srcs = glob(["src/*.h"]),
    visibility = ["//visibility:public"],
)

# Template files that are included by cr*.c and fft*.c
filegroup(
    name = "soxr_templates",
    srcs = [
        "src/cr-core.c",
        "src/cr.c",
        "src/fft4g.c",
    ],
    visibility = ["//visibility:public"],
)

exports_files(
    ["src/soxr.h"],
    visibility = ["//visibility:public"],
)
