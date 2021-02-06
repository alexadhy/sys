local dbcfg = import "vendor/github.com/amplify-edge/sys-share/sys-core/service/config/mixins/mixin.db.libsonnet";

{
    local cfg = self,
    CoreDB:: dbcfg.DB {
      name: "core.db",
    },
    CoreCron:: dbcfg.Cron,
    CoreMail:: {
      senderName: "gutterbacon",
      senderMail: "gutterbacon@example.com",
      productName: "SOME_PRODUCT",
      logoUrl: "https://via.placeholder.com/500x500?text=YOUR+LOGO+HERE",
      copyright: "SOME_COPYRIGHT_MSG",
      troubleContact: "SOME_TROUBLESHOOT_CONTACT_HERE",
      sendgrid: {
        apiKey: "SENDGRID_API_KEY_HERE",
      },
      smtp: {
        host: "smtp.gmail.com",
        port: 587,
        email: "winwisely268@gmail.com",
        password: "blablabla",
      }
    },
    sysCoreConfig: {
        db: self.CoreDB,
        cron: self.CoreCron,
    },
    mailConfig: self.CoreMail,
}