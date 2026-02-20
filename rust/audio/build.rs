fn main() {
    println!("cargo:rustc-link-search=native=/opt/homebrew/lib");
    println!("cargo:rustc-link-lib=opus");
    println!("cargo:rustc-link-lib=mp3lame");
    println!("cargo:rustc-link-lib=portaudio");

    // minimp3 is header-only, compiled from source
    cc::Build::new()
        .file("../../third_party/minimp3/minimp3.c")
        .include("../../third_party/minimp3")
        .compile("minimp3");
}
