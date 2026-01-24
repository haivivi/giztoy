@Entry
@Component
struct Index {
  build() {
    RelativeContainer() {
      Column() {
        Text('预览失败')
          .fontSize(12)
          .fontWeight(FontWeight.Bold)
          .textAlign(TextAlign.Center)
        Text('无法启动预览器。请点击刷新按钮重新加载，或点击"保存到工程"，然后手动启动预览器。')
          .fontSize(12)
          .textAlign(TextAlign.Center)
        Text('Preview failed')
          .fontSize(12)
          .fontWeight(FontWeight.Bold)
          .textAlign(TextAlign.Center)
        Text('Unable to start the previewer. Click the refresh button to reload, or click "Save to Project", then restart the previewer.')
          .fontSize(12)
          .textAlign(TextAlign.Center)
      }
      .justifyContent(FlexAlign.Center)
      .alignItems(HorizontalAlign.Center)
      .width('100%')
      .height('100%')
    }
    .height('100%')
    .width('100%')
  }
}