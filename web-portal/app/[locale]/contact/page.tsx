import type { Metadata } from "next";
import { Mail } from "lucide-react";
import { getTranslations, setRequestLocale } from "next-intl/server";
import { ContactForm } from "@/components/contact-form";
import { GitHubIcon } from "@/components/github-icon";
import { PageHeader } from "@/components/page-header";
import { pageMetadata } from "@/lib/seo";
import { CONTACT_EMAIL, GITHUB_URL } from "@/lib/site";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "meta" });
  return pageMetadata({ locale, path: "/contact", title: t("pages.contact"), description: t("descriptions.contact"), ogTitleEn: "Contact" });
}

export default async function Contact({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations("contact");
  return (
    <>
      <PageHeader title={t("title")} intro={t("intro")} />
      <div className="mx-auto grid max-w-7xl gap-8 px-4 sm:px-6 lg:grid-cols-[1.4fr_1fr]">
        <ContactForm />
        <aside aria-labelledby="c-ways" className="space-y-4">
          <h2 id="c-ways" className="text-lg font-semibold">{t("ways.title")}</h2>
          <a href={`mailto:${CONTACT_EMAIL}`} className="glass glass-hover flex items-center gap-3 p-4">
            <span className="icon-tile"><Mail className="h-5 w-5" aria-hidden /></span>
            <span><span className="block font-medium">{t("ways.email")}</span><span className="text-sm text-muted-foreground [overflow-wrap:anywhere]">{CONTACT_EMAIL}</span></span>
          </a>
          <a href={`${GITHUB_URL}/issues/new`} target="_blank" rel="noopener noreferrer" className="glass glass-hover flex items-center gap-3 p-4">
            <span className="icon-tile"><GitHubIcon className="h-5 w-5" /></span>
            <span><span className="block font-medium">{t("ways.github")}</span><span className="text-sm text-muted-foreground">{t("ways.githubText")}</span></span>
          </a>
          <p className="text-sm text-muted-foreground">{t("securityNote")}</p>
        </aside>
      </div>
    </>
  );
}
