@Entry
@Component
struct Index {
  @State message: string = 'Hello Bazel!'
  @State count: number = 0

  build() {
    Column() {
      Text(this.message)
        .fontSize(50)
        .fontWeight(FontWeight.Bold)
        .fontColor('#007DFF')
        .margin({ top: 100 })

      Text(`Built with Bazel + hvigorw`)
        .fontSize(20)
        .fontColor('#666666')
        .margin({ top: 20 })

      Text(`Count: ${this.count}`)
        .fontSize(30)
        .margin({ top: 50 })

      Button('Click Me!')
        .fontSize(24)
        .width(200)
        .height(60)
        .margin({ top: 30 })
        .onClick(() => {
          this.count++
        })
    }
    .width('100%')
    .height('100%')
    .justifyContent(FlexAlign.Start)
    .alignItems(HorizontalAlign.Center)
  }
}
