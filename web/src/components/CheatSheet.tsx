import { type ReactNode } from 'react';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { useT } from '@/lib/i18n';

function GuideSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-2 border-b border-border pb-5 last:border-0">
      <h3 className="text-sm font-semibold text-foreground">{title}</h3>
      <div className="space-y-2 text-sm leading-5 text-muted-foreground">{children}</div>
    </section>
  );
}

function GuideItem({ term, children }: { term: string; children: ReactNode }) {
  return <p><span className="font-medium text-foreground">{term}</span> — {children}</p>;
}

export function CheatSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const t = useT();
  return (
    <Sheet open={open} onOpenChange={(nextOpen) => { if (!nextOpen) onClose(); }}>
      <SheetContent className="w-full gap-0 p-0 sm:max-w-xl" aria-label="Cheat sheet">
        <SheetHeader className="border-b border-border px-5 py-4 pr-12 text-left">
          <SheetTitle className="text-base">{t('Cheat sheet: how to read the analysis', 'Cheat sheet: jak czytać analizę')}</SheetTitle>
          <SheetDescription>{t('Use signals to choose what to review in a demo — never as an automatic verdict.', 'Używaj sygnałów, aby zdecydować, co sprawdzić w demie — nigdy jako automatycznego werdyktu.')}</SheetDescription>
        </SheetHeader>
        <div className="cheat-sheet-scroll min-h-0 flex-1 space-y-5 overflow-y-auto px-5 py-5">
          <GuideSection title={t('Start here', 'Zacznij tutaj')}>
            <GuideItem term="Status">{t('A review priority, not proof. `Cheater` (red) marks stats that are not humanly reproducible over many games; `Watch` (yellow) is a grey-zone flag. Always expand the row and inspect the reason badges below the player stats before judging.', 'To priorytet do sprawdzenia, a nie dowód. `Cheater` (czerwony) oznacza statystyki niemożliwe do powtórzenia przez człowieka w wielu meczach; `Watch` (żółty) to flaga ze strefy niepewności. Zawsze rozwiń wiersz i sprawdź plakietki z powodami pod statystykami gracza, zanim ocenisz.')}</GuideItem>
            <GuideItem term="Score">{t('A conservative 0–100 review score, not a probability of cheating. An unusual timing signal is required. Accuracy, head-hit and K/D can only add bounded support because excellent legitimate players can score highly on all three; they never create a flag alone. Correlated metrics inside a group are not added together.', 'Konserwatywny wynik 0–100 do przeglądu, a nie prawdopodobieństwo cheatowania. Wymagany jest nietypowy sygnał czasowy. Accuracy, head-hit i K/D mogą dodać jedynie ograniczone wsparcie, bo świetni legalni gracze potrafią osiągać wysokie wartości we wszystkich trzech; nigdy same nie tworzą flagi. Skorelowane metryki w jednej grupie nie są sumowane.')}</GuideItem>
            <GuideItem term="Samples">{t('Check the number of demos, shots and sample count (`n=`). A small sample can make an ordinary streak look extreme.', 'Sprawdź liczbę dem, strzałów i wielkość próbki (`n=`). Mała próbka może sprawić, że zwykła seria wygląda ekstremalnie.')}</GuideItem>
            <GuideItem term="Workflow">{t('Use the table to find a player, open their row, note multiple independent signals, then review the relevant rounds in the demo.', 'Użyj tabeli, aby znaleźć gracza, rozwiń jego wiersz, odnotuj kilka niezależnych sygnałów, a potem obejrzyj odpowiednie rundy w demie.')}</GuideItem>
          </GuideSection>

          <GuideSection title={t('Basic combat context', 'Podstawowy kontekst walki')}>
            <GuideItem term="Demos / shots">{t('The amount of evidence available. More unique demos and weapon-fire events make a comparison more useful.', 'Ilość dostępnego materiału. Więcej unikalnych dem i zdarzeń oddania strzału czyni porównanie bardziej wiarygodnym.')}</GuideItem>
            <GuideItem term="Kills / deaths">{t('Impact context only. A high K/D is never, by itself, evidence of cheating.', 'Wyłącznie kontekst wpływu na grę. Wysokie K/D samo w sobie nigdy nie jest dowodem cheatowania.')}</GuideItem>
            <GuideItem term="Accuracy">{t('Tracked hit shots divided by tracked weapon shots. Compare weapon choice and sample size; AWP, pistols and rifles behave differently.', 'Zarejestrowane trafienia podzielone przez zarejestrowane strzały. Porównaj wybór broni i wielkość próbki; AWP, pistolety i karabiny zachowują się inaczej.')}</GuideItem>
            <GuideItem term="Head hit / HS kills">{t('Head-hit rate uses every damaging hit; HS-kill rate uses only finishing headshots. High values can be skill, weapon choice or close-range play.', 'Head-hit rate liczy każde trafienie zadające obrażenia; HS-kill rate liczy tylko dobijające headshoty. Wysokie wartości mogą wynikać z umiejętności, wyboru broni lub gry z bliska.')}</GuideItem>
          </GuideSection>

          <GuideSection title={t('Exposure and response', 'Odsłonięcie i reakcja')}>
            <GuideItem term="TTD (rifle)">{t('Time from first spotted tick to first damage on non-AWP weapons, as a round-weighted long-term average. Lower values create progressively stronger timing evidence between the configurable anchors; they no longer act as a binary verdict. `p10 (all)` in details is the fast 10% tail across every weapon.', 'Czas od pierwszego ticku spotted do pierwszych obrażeń bronią inną niż AWP, jako długoterminowa średnia ważona rundami. Niższe wartości stopniowo tworzą silniejszy dowód czasowy między konfigurowalnymi progami; nie działają już jak binarny werdykt. `p10 (all)` w szczegółach to szybkie 10% ogona dla wszystkich broni.')}</GuideItem>
            <GuideItem term="TTD (AWP)">{t('AWP-only TTD. The AWP is a one-flick one-shot weapon and is often used while holding an angle, so it has lower anchors and a separate evidence weight. Only the strongest timing metric contributes to the score.', 'TTD tylko dla AWP. AWP to broń typu one-flick one-shot, często używana przy trzymaniu kąta, więc ma niższe progi i osobną wagę dowodu. Do wyniku wnosi tylko najsilniejsza metryka czasowa.')}</GuideItem>
            <GuideItem term="Reaction (rifle)">{t('Time from first spotted tick to first shot on non-AWP weapons. It is a demo-derived estimate, not a laboratory reaction-time test; pre-aim, sound cues and prediction affect it. It shares the timing group with TTD.', 'Czas od pierwszego ticku spotted do pierwszego strzału bronią inną niż AWP. To szacunek z dema, a nie laboratoryjny test czasu reakcji; wpływają na niego pre-aim, dźwięki i przewidywanie. Dzieli grupę czasową z TTD.')}</GuideItem>
            <GuideItem term="Crosshair @ exposure">{t('Median angular distance from crosshair to opponent at confirmed exposure. Lower means stronger crosshair placement, not cheating by itself.', 'Mediana odległości kątowej celownika od przeciwnika w momencie potwierdzonego odsłonięcia. Niższa wartość oznacza lepsze ustawienie celownika, a nie cheaty same w sobie.')}</GuideItem>
            <GuideItem term="First shot error">{t('Median angular distance at the first shot. Read it together with TTD and reaction instead of treating it as a standalone verdict.', 'Mediana odległości kątowej przy pierwszym strzale. Czytaj ją razem z TTD i reakcją, a nie jako samodzielny werdykt.')}</GuideItem>
          </GuideSection>

          <GuideSection title={t('Suspicion signals', 'Sygnały podejrzeń')}>
            <GuideItem term="Unspotted damage">{t('Damage where the analyzer did not have a confirmed spotted state. Check the demo for sound, teammate information, wallbang lines, smokes and replay limitations before judging it.', 'Obrażenia zadane, gdy analizator nie miał potwierdzonego stanu spotted. Sprawdź w demie dźwięk, informacje od kolegów z drużyny, linie wallbangów, smoke\u2019y i ograniczenia powtórki, zanim ocenisz.')}</GuideItem>
            <GuideItem term="First-bullet head / snap">{t('Signals around unusually accurate first shots and fast aim reduction. They are strongest when repeated over many encounters and paired with unusual TTD or reactions.', 'Sygnały dotyczące nietypowo celnych pierwszych strzałów i szybkiego naprowadzania celownika. Są najsilniejsze, gdy powtarzają się w wielu starciach i łączą z nietypowym TTD lub reakcją.')}</GuideItem>
            <GuideItem term="Smoke / wall kills">{t('Useful review context, but legitimate wallbangs and common angles are expected in Counter-Strike. Look for repetition and timing, not isolated kills.', 'Przydatny kontekst do przeglądu, ale legalne wallbangi i typowe kąty są w Counter-Strike normalne. Szukaj powtarzalności i wyczucia czasu, a nie pojedynczych zabójstw.')}</GuideItem>
            <GuideItem term="Reason badges">{t('Coloured badges below the player stats show the strongest metric from each evidence group that contributed to the final Watch or Cheater score.', 'Kolorowe plakietki pod statystykami gracza pokazują najsilniejszą metrykę z każdej grupy dowodów, która wpłynęła na końcowy wynik Watch lub Cheater.')}</GuideItem>
          </GuideSection>

          <GuideSection title={t('Saved players and demos', 'Zapisani gracze i dema')}>
            <GuideItem term="Bookmark">{t('Use the bookmark in the Status column to pin a player to the top while reviewing them later.', 'Użyj zakładki w kolumnie Status, aby przypiąć gracza na górze, gdy chcesz go sprawdzić później.')}</GuideItem>
            <GuideItem term="Demos">{t('The Demos page lets you enable or disable demos from aggregates, manage imports and exports, and clear uploaded files after analysis. A quality warning automatically excludes a demo when low timing affects several players; you can still include it manually after review.', 'Strona Dema pozwala włączać i wyłączać dema z zestawień, zarządzać importem i eksportem oraz usuwać wgrane pliki po analizie. Ostrzeżenie o jakości automatycznie wyklucza demo, gdy niskie czasy dotyczą wielu graczy; nadal możesz je włączyć ręcznie po przeglądzie.')}</GuideItem>
          </GuideSection>

          <GuideSection title={t('Decision rule', 'Zasada decyzji')}>
            <p>{t('One metric is a reason to look, not a conclusion. Prioritize players with enough samples and multiple independent signals, then validate them against the demo timeline.', 'Jedna metryka to powód, by się przyjrzeć, a nie wniosek. Traktuj priorytetowo graczy z wystarczającą próbką i kilkoma niezależnymi sygnałami, a następnie zweryfikuj ich na osi czasu dema.')}</p>
          </GuideSection>
        </div>
      </SheetContent>
    </Sheet>
  );
}
