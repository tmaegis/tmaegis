import { getTranslations } from "next-intl/server";
import { SearchX } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Link } from "@/i18n/navigation";

export default async function NotFound() {
  const t = await getTranslations("notFound");
  return (
    <div className="mx-auto flex max-w-2xl flex-col items-center px-4 py-28 text-center">
      <span className="icon-tile h-14 w-14" aria-hidden><SearchX className="h-7 w-7" /></span>
      <p className="gradient-text mt-6 font-mono text-7xl font-bold" aria-hidden>404</p>
      <h1 className="mt-4 text-3xl font-semibold tracking-tight">{t("title")}</h1>
      <p className="mt-3 text-muted-foreground">{t("text")}</p>
      <div className="mt-8 flex flex-wrap justify-center gap-3">
        <Button asChild><Link href="/">{t("home")}</Link></Button>
        <Button asChild variant="outline"><Link href="/rules">{t("rules")}</Link></Button>
      </div>
    </div>
  );
}
