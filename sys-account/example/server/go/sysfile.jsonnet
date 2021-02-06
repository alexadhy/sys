local coreTpl = import "../../sys-core/service/go/template.syscore.libsonnet";
local loadVar = import "vendor/github.com/amplify-edge/sys-share/sys-core/service/config/mixins/mixin.loadfn.libsonnet";

local cfg = {
   dbConfig: {
       db: coreTpl.CoreDB {
         name: "files.db",
         encryptKey: loadVar(prefixName="SYS_FILE", env="DB_ENCRYPT_KEY").val,
         dbDir: "./db",
         deletePrevious: true,
       },
       cron: coreTpl.CoreCron {
         backupSchedule: "@daily",
       },
   }
};

std.manifestYamlDoc(cfg)