package domain

import (
	"slices"
	"strings"
)

// MoscowZone описывает зону доставки в Москве.
type MoscowZone struct {
	ID    string
	Okrug string
	Name  string
}

var moscowZones = []MoscowZone{
	{ID: "msk-cao-arbat", Okrug: "cao", Name: "Arbat"},
	{ID: "msk-cao-basmanny", Okrug: "cao", Name: "Basmanny"},
	{ID: "msk-cao-zamoskvorechye", Okrug: "cao", Name: "Zamoskvorechye"},
	{ID: "msk-cao-krasnoselsky", Okrug: "cao", Name: "Krasnoselsky"},
	{ID: "msk-cao-meshchansky", Okrug: "cao", Name: "Meshchansky"},
	{ID: "msk-cao-presnensky", Okrug: "cao", Name: "Presnensky"},
	{ID: "msk-cao-tagansky", Okrug: "cao", Name: "Tagansky"},
	{ID: "msk-cao-tverskoy", Okrug: "cao", Name: "Tverskoy"},
	{ID: "msk-cao-khamovniki", Okrug: "cao", Name: "Khamovniki"},
	{ID: "msk-cao-yakimanka", Okrug: "cao", Name: "Yakimanka"},

	{ID: "msk-sao-aeroport", Okrug: "sao", Name: "Aeroport"},
	{ID: "msk-sao-begovoy", Okrug: "sao", Name: "Begovoy"},
	{ID: "msk-sao-beskudnikovsky", Okrug: "sao", Name: "Beskudnikovsky"},
	{ID: "msk-sao-voykovsky", Okrug: "sao", Name: "Voykovsky"},
	{ID: "msk-sao-vostochnoe-degunino", Okrug: "sao", Name: "Vostochnoe-Degunino"},
	{ID: "msk-sao-golovinsky", Okrug: "sao", Name: "Golovinsky"},
	{ID: "msk-sao-dmitrovsky", Okrug: "sao", Name: "Dmitrovsky"},
	{ID: "msk-sao-zapadnoe-degunino", Okrug: "sao", Name: "Zapadnoe-Degunino"},
	{ID: "msk-sao-koptevo", Okrug: "sao", Name: "Koptevo"},
	{ID: "msk-sao-levoberezhny", Okrug: "sao", Name: "Levoberezhny"},
	{ID: "msk-sao-molzhaninovsky", Okrug: "sao", Name: "Molzhaninovsky"},
	{ID: "msk-sao-savelovsky", Okrug: "sao", Name: "Savelovsky"},
	{ID: "msk-sao-sokol", Okrug: "sao", Name: "Sokol"},
	{ID: "msk-sao-timiryazevsky", Okrug: "sao", Name: "Timiryazevsky"},
	{ID: "msk-sao-khovrino", Okrug: "sao", Name: "Khovrino"},
	{ID: "msk-sao-khoroshevsky", Okrug: "sao", Name: "Khoroshevsky"},

	{ID: "msk-svao-alekseevsky", Okrug: "svao", Name: "Alekseevsky"},
	{ID: "msk-svao-altufyevsky", Okrug: "svao", Name: "Altufyevsky"},
	{ID: "msk-svao-babushkinsky", Okrug: "svao", Name: "Babushkinsky"},
	{ID: "msk-svao-bibirevo", Okrug: "svao", Name: "Bibirevo"},
	{ID: "msk-svao-butyrsky", Okrug: "svao", Name: "Butyrsky"},
	{ID: "msk-svao-lianozovo", Okrug: "svao", Name: "Lianozovo"},
	{ID: "msk-svao-losinoostrovsky", Okrug: "svao", Name: "Losinoostrovsky"},
	{ID: "msk-svao-marfino", Okrug: "svao", Name: "Marfino"},
	{ID: "msk-svao-maryina-roshcha", Okrug: "svao", Name: "Maryina-Roshcha"},
	{ID: "msk-svao-ostankinsky", Okrug: "svao", Name: "Ostankinsky"},
	{ID: "msk-svao-otradnoe", Okrug: "svao", Name: "Otradnoe"},
	{ID: "msk-svao-rostokino", Okrug: "svao", Name: "Rostokino"},
	{ID: "msk-svao-sviblovo", Okrug: "svao", Name: "Sviblovo"},
	{ID: "msk-svao-severny", Okrug: "svao", Name: "Severny"},
	{ID: "msk-svao-severnoe-medvedkovo", Okrug: "svao", Name: "Severnoe-Medvedkovo"},
	{ID: "msk-svao-yuzhnoe-medvedkovo", Okrug: "svao", Name: "Yuzhnoe-Medvedkovo"},
	{ID: "msk-svao-yaroslavsky", Okrug: "svao", Name: "Yaroslavsky"},

	{ID: "msk-vao-bogorodskoe", Okrug: "vao", Name: "Bogorodskoe"},
	{ID: "msk-vao-veshnyaki", Okrug: "vao", Name: "Veshnyaki"},
	{ID: "msk-vao-vostochny", Okrug: "vao", Name: "Vostochny"},
	{ID: "msk-vao-vostochnoe-izmaylovo", Okrug: "vao", Name: "Vostochnoe-Izmaylovo"},
	{ID: "msk-vao-golyanovo", Okrug: "vao", Name: "Golyanovo"},
	{ID: "msk-vao-ivanovskoe", Okrug: "vao", Name: "Ivanovskoe"},
	{ID: "msk-vao-izmaylovo", Okrug: "vao", Name: "Izmaylovo"},
	{ID: "msk-vao-kosino-ukhtomsky", Okrug: "vao", Name: "Kosino-Ukhtomsky"},
	{ID: "msk-vao-metrogorodok", Okrug: "vao", Name: "Metrogorodok"},
	{ID: "msk-vao-novogireevo", Okrug: "vao", Name: "Novogireevo"},
	{ID: "msk-vao-novokosino", Okrug: "vao", Name: "Novokosino"},
	{ID: "msk-vao-perovo", Okrug: "vao", Name: "Perovo"},
	{ID: "msk-vao-preobrazhenskoe", Okrug: "vao", Name: "Preobrazhenskoe"},
	{ID: "msk-vao-severnoe-izmaylovo", Okrug: "vao", Name: "Severnoe-Izmaylovo"},
	{ID: "msk-vao-sokolinaya-gora", Okrug: "vao", Name: "Sokolinaya-Gora"},
	{ID: "msk-vao-sokolniki", Okrug: "vao", Name: "Sokolniki"},

	{ID: "msk-yuvao-vykhino-zhulebino", Okrug: "yuvao", Name: "Vykhino-Zhulebino"},
	{ID: "msk-yuvao-kapotnya", Okrug: "yuvao", Name: "Kapotnya"},
	{ID: "msk-yuvao-kuzminki", Okrug: "yuvao", Name: "Kuzminki"},
	{ID: "msk-yuvao-lefortovo", Okrug: "yuvao", Name: "Lefortovo"},
	{ID: "msk-yuvao-lyublino", Okrug: "yuvao", Name: "Lyublino"},
	{ID: "msk-yuvao-maryino", Okrug: "yuvao", Name: "Maryino"},
	{ID: "msk-yuvao-nekrasovka", Okrug: "yuvao", Name: "Nekrasovka"},
	{ID: "msk-yuvao-nizhegorodsky", Okrug: "yuvao", Name: "Nizhegorodsky"},
	{ID: "msk-yuvao-pechatniki", Okrug: "yuvao", Name: "Pechatniki"},
	{ID: "msk-yuvao-ryazansky", Okrug: "yuvao", Name: "Ryazansky"},
	{ID: "msk-yuvao-tekstilshchiki", Okrug: "yuvao", Name: "Tekstilshchiki"},
	{ID: "msk-yuvao-yuzhnoportovy", Okrug: "yuvao", Name: "Yuzhnoportovy"},

	{ID: "msk-yuao-biryulyovo-vostochnoe", Okrug: "yuao", Name: "Biryulyovo-Vostochnoe"},
	{ID: "msk-yuao-biryulyovo-zapadnoe", Okrug: "yuao", Name: "Biryulyovo-Zapadnoe"},
	{ID: "msk-yuao-brateevo", Okrug: "yuao", Name: "Brateevo"},
	{ID: "msk-yuao-danilovsky", Okrug: "yuao", Name: "Danilovsky"},
	{ID: "msk-yuao-donskoy", Okrug: "yuao", Name: "Donskoy"},
	{ID: "msk-yuao-zyablikovo", Okrug: "yuao", Name: "Zyablikovo"},
	{ID: "msk-yuao-moskvorechye-saburovo", Okrug: "yuao", Name: "Moskvorechye-Saburovo"},
	{ID: "msk-yuao-nagatino-sadovniki", Okrug: "yuao", Name: "Nagatino-Sadovniki"},
	{ID: "msk-yuao-nagatinsky-zaton", Okrug: "yuao", Name: "Nagatinsky-Zaton"},
	{ID: "msk-yuao-nagorny", Okrug: "yuao", Name: "Nagorny"},
	{ID: "msk-yuao-orekhovo-borisovo-severnoe", Okrug: "yuao", Name: "Orekhovo-Borisovo-Severnoe"},
	{ID: "msk-yuao-orekhovo-borisovo-yuzhnoe", Okrug: "yuao", Name: "Orekhovo-Borisovo-Yuzhnoe"},
	{ID: "msk-yuao-tsaritsyno", Okrug: "yuao", Name: "Tsaritsyno"},
	{ID: "msk-yuao-chertanovo-severnoe", Okrug: "yuao", Name: "Chertanovo-Severnoe"},
	{ID: "msk-yuao-chertanovo-tsentralnoe", Okrug: "yuao", Name: "Chertanovo-Tsentralnoe"},
	{ID: "msk-yuao-chertanovo-yuzhnoe", Okrug: "yuao", Name: "Chertanovo-Yuzhnoe"},

	{ID: "msk-yuzao-akademichesky", Okrug: "yuzao", Name: "Akademichesky"},
	{ID: "msk-yuzao-gagarinsky", Okrug: "yuzao", Name: "Gagarinsky"},
	{ID: "msk-yuzao-zyuzino", Okrug: "yuzao", Name: "Zyuzino"},
	{ID: "msk-yuzao-konkovo", Okrug: "yuzao", Name: "Konkovo"},
	{ID: "msk-yuzao-kotlovka", Okrug: "yuzao", Name: "Kotlovka"},
	{ID: "msk-yuzao-lomonosovsky", Okrug: "yuzao", Name: "Lomonosovsky"},
	{ID: "msk-yuzao-obruchevsky", Okrug: "yuzao", Name: "Obruchevsky"},
	{ID: "msk-yuzao-severnoe-butovo", Okrug: "yuzao", Name: "Severnoe-Butovo"},
	{ID: "msk-yuzao-teply-stan", Okrug: "yuzao", Name: "Teply-Stan"},
	{ID: "msk-yuzao-cheryomushki", Okrug: "yuzao", Name: "Cheryomushki"},
	{ID: "msk-yuzao-yuzhnoe-butovo", Okrug: "yuzao", Name: "Yuzhnoe-Butovo"},
	{ID: "msk-yuzao-yasenevo", Okrug: "yuzao", Name: "Yasenevo"},

	{ID: "msk-zao-dorogomilovo", Okrug: "zao", Name: "Dorogomilovo"},
	{ID: "msk-zao-krylatskoe", Okrug: "zao", Name: "Krylatskoe"},
	{ID: "msk-zao-kuntsevo", Okrug: "zao", Name: "Kuntsevo"},
	{ID: "msk-zao-mozhaysky", Okrug: "zao", Name: "Mozhaysky"},
	{ID: "msk-zao-novo-peredelkino", Okrug: "zao", Name: "Novo-Peredelkino"},
	{ID: "msk-zao-ochakovo-matveevskoe", Okrug: "zao", Name: "Ochakovo-Matveevskoe"},
	{ID: "msk-zao-prospekt-vernadskogo", Okrug: "zao", Name: "Prospekt-Vernadskogo"},
	{ID: "msk-zao-ramenki", Okrug: "zao", Name: "Ramenki"},
	{ID: "msk-zao-solntsevo", Okrug: "zao", Name: "Solntsevo"},
	{ID: "msk-zao-troparevo-nikulino", Okrug: "zao", Name: "Troparevo-Nikulino"},
	{ID: "msk-zao-filevsky-park", Okrug: "zao", Name: "Filevsky-Park"},
	{ID: "msk-zao-fili-davydkovo", Okrug: "zao", Name: "Fili-Davydkovo"},

	{ID: "msk-szao-kurkino", Okrug: "szao", Name: "Kurkino"},
	{ID: "msk-szao-mitino", Okrug: "szao", Name: "Mitino"},
	{ID: "msk-szao-pokrovskoe-streshnevo", Okrug: "szao", Name: "Pokrovskoe-Streshnevo"},
	{ID: "msk-szao-severnoe-tushino", Okrug: "szao", Name: "Severnoe-Tushino"},
	{ID: "msk-szao-strogino", Okrug: "szao", Name: "Strogino"},
	{ID: "msk-szao-khoroshevo-mnevniki", Okrug: "szao", Name: "Khoroshevo-Mnevniki"},
	{ID: "msk-szao-shchukino", Okrug: "szao", Name: "Shchukino"},
	{ID: "msk-szao-yuzhnoe-tushino", Okrug: "szao", Name: "Yuzhnoe-Tushino"},

	{ID: "msk-zelao-matushkino", Okrug: "zelao", Name: "Matushkino"},
	{ID: "msk-zelao-savelki", Okrug: "zelao", Name: "Savelki"},
	{ID: "msk-zelao-staroe-kryukovo", Okrug: "zelao", Name: "Staroe-Kryukovo"},
	{ID: "msk-zelao-silino", Okrug: "zelao", Name: "Silino"},
	{ID: "msk-zelao-kryukovo", Okrug: "zelao", Name: "Kryukovo"},

	{ID: "msk-nao-vnukovo", Okrug: "nao", Name: "Vnukovo"},
	{ID: "msk-nao-kommunarka", Okrug: "nao", Name: "Kommunarka"},
	{ID: "msk-nao-filimonkovsky", Okrug: "nao", Name: "Filimonkovsky"},
	{ID: "msk-nao-shcherbinka", Okrug: "nao", Name: "Shcherbinka"},

	{ID: "msk-tao-bekasovo", Okrug: "tao", Name: "Bekasovo"},
	{ID: "msk-tao-voronovo", Okrug: "tao", Name: "Voronovo"},
	{ID: "msk-tao-krasnopakhorsky", Okrug: "tao", Name: "Krasnopakhorsky"},
	{ID: "msk-tao-troitsk", Okrug: "tao", Name: "Troitsk"},
}

var moscowZoneSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(moscowZones))
	for _, zone := range moscowZones {
		set[zone.ID] = struct{}{}
	}
	return set
}()

// NormalizeZoneID приводит идентификатор зоны к каноническому виду.
func NormalizeZoneID(zoneID string) string {
	return strings.ToLower(strings.TrimSpace(zoneID))
}

// IsKnownMoscowZoneID проверяет, зарегистрирован ли zone_id в каталоге Москвы.
func IsKnownMoscowZoneID(zoneID string) bool {
	_, ok := moscowZoneSet[NormalizeZoneID(zoneID)]
	return ok
}

// MoscowZonesCatalog возвращает отсортированную копию справочника зон Москвы.
func MoscowZonesCatalog() []MoscowZone {
	result := make([]MoscowZone, len(moscowZones))
	copy(result, moscowZones)
	slices.SortFunc(result, func(a, b MoscowZone) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return result
}
