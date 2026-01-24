import hilog from "@ohos:hilog";
import BackupExtensionAbility from "@ohos:application.BackupExtensionAbility";
import type { BundleVersion } from "@ohos:application.BackupExtensionAbility";
const DOMAIN = 0x0000;
export default class EntryBackupAbility extends BackupExtensionAbility {
    async onBackup() {
        hilog.info(DOMAIN, 'testTag', 'onBackup ok');
        await Promise.resolve();
    }
    async onRestore(bundleVersion: BundleVersion) {
        hilog.info(DOMAIN, 'testTag', 'onRestore ok %{public}s', JSON.stringify(bundleVersion));
        await Promise.resolve();
    }
}
