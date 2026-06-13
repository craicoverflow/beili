package scaling

import "testing"

func TestScaleIngredient(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		ratio float64
		want  string
	}{
		// --- ASCII fractions (Pinch of Yum cookies, raw imperial) ---
		{"half cup doubles", "1/2 cup white sugar (I like to use raw cane sugar)", 2, "1 cup white sugar (I like to use raw cane sugar)"},
		{"quarter cup doubles", "1/4 cup packed light brown sugar", 2, "½ cup packed light brown sugar"},
		{"mixed number with parenthetical ounces", "1 1/2 cups all purpose flour (6.75 ounces)", 2, "3 cups all purpose flour (13.5 ounces)"},
		{"half teaspoon doubles", "1/2 teaspoon baking soda", 2, "1 teaspoon baking soda"},
		{"quarter teaspoon with note", "1/4 teaspoon salt (but I always add a little extra)", 2, "½ teaspoon salt (but I always add a little extra)"},
		{"three quarter cup doubles", "3/4 cup chocolate chips", 2, "1½ cups chocolate chips"},
		{"full word tablespoons", "8 tablespoons of salted butter", 2, "16 tablespoons of salted butter"},
		{"tablespoons scaled down", "8 tablespoons of salted butter", 5.0 / 12, "3¼ tablespoons of salted butter"},
		{"single egg doubles", "1 egg", 2, "2 eggs"},
		{"vanilla teaspoon", "1 teaspoon vanilla", 2, "2 teaspoons vanilla"},

		// --- unicode fractions ---
		{"vulgar half cup", "½ cup sugar", 3, "1½ cups sugar"},
		{"vulgar third cup", "⅓ cup olive oil", 2, "¾ cup olive oil"},
		{"attached mixed vulgar", "1½ tsp paprika", 2, "3 tsp paprika"},
		{"vulgar range", "½-1 tsp chilli flakes", 2, "1-2 tsp chilli flakes"},

		// --- RecipeTin Eats butter chicken (raw) ---
		{"dual imperial metric", "1.5 lb / 750 g chicken thigh fillets, cut into bite size pieces", 2, "3 lb / 1.5 kg chicken thigh fillets, cut into bite size pieces"},
		{"tbsp with grams and note", "2 tbsp (30 g) ghee or butter, ( OR 1 tbsp vegetable oil (Note 3))", 2, "4 tbsp (60 g) ghee or butter, ( OR 2 tbsp vegetable oil (Note 3))"},
		{"mixed fraction tsp", "1 1/4 tsp salt", 2, "2½ tsp salt"},
		{"cloves scaled by thirds", "2 cloves garlic, crushed", 5.0 / 3, "3½ cloves garlic, crushed"},
		{"unquantified line untouched", "Basmati rice", 2, "Basmati rice"},
		{"optional garnish untouched", "Coriander/cilantro (optional)", 2, "Coriander/cilantro (optional)"},

		// --- metric (AI-normalised recipes) ---
		{"grams upgrade to kg", "940 g lean beef mince", 2, "1.9 kg lean beef mince"},
		{"kg downgrades to g", "1.0 kg passata or half our basic tomato sauce", 0.5, "500 g passata or half our basic tomato sauce"},
		{"ml round to ten", "250 ml hot beef stock", 1.2, "300 ml hot beef stock"},
		{"small ml rounds to five", "40 ml olive oil plus extra for the dish", 1.2, "50 ml olive oil plus extra for the dish"},
		{"tiny ml rounds to half", "8 ml tumeric powder", 1.2, "9.5 ml tumeric powder"},
		{"pack size kept with multiplier", "110 g pack prosciutto", 2, "2 x 110 g pack prosciutto"},
		{"can size kept with multiplier", "400 g can chopped tomatoes", 1.5, "1½ x 400 g can chopped tomatoes"},
		{"existing multiplier scales not size", "2 x 400 g cans chopped tomatoes", 2, "4 x 400 g cans chopped tomatoes"},

		// --- bare counts and ranges ---
		{"bare count eggs", "5 large eggs", 1.4, "7 large eggs"},
		{"count range", "2 or 3 green onions, sliced", 2, "4 or 6 green onions, sliced"},
		{"handful pluralises", "1 handful fresh cilantro (optional)", 2, "2 handfuls fresh cilantro (optional)"},
		{"onion singularises", "2 onions, finely chopped", 0.5, "1 onion, finely chopped"},
		{"fractional count", "1 onion", 1.5, "1½ onions"},
		{"about prefix still leading", "about 4 carrots, sliced", 0.5, "about 2 carrots, sliced"},
		{"dash range", "4-5 tomatoes", 2, "8-10 tomatoes"},

		// --- guards: things that must never scale ---
		{"length in ingredient", "2 in piece of ginger", 2, "2 in piece of ginger"},
		{"cm cake tin", "1 x 20cm cake tin", 2, "2 x 20cm cake tin"},
		{"percentage untouched", "85% lean beef mince", 2, "85% lean beef mince"},

		{"quarter cup halves to eighth", "1/4 cup packed light brown sugar", 0.5, "⅛ cup packed light brown sugar"},

		// --- identity ---
		{"ratio one unchanged", "1/2 cup white sugar", 1, "1/2 cup white sugar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScaleIngredient(tt.in, tt.ratio); got != tt.want {
				t.Errorf("ScaleIngredient(%q, %v)\n got:  %q\n want: %q", tt.in, tt.ratio, got, tt.want)
			}
		})
	}
}

func TestScaleInstruction(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		ratio float64
		want  string
	}{
		// temperatures, times, gas marks, dimensions stay fixed
		{"oven line untouched", "Heat the oven to 180C/160C fan/gas 4 and lightly oil an ovenproof dish (about 30 x 20cm).", 2,
			"Heat the oven to 180C/160C fan/gas 4 and lightly oil an ovenproof dish (about 30 x 20cm)."},
		{"bake time untouched", "Bake for 45 mins until the top is bubbling and lightly browned.", 2,
			"Bake for 45 mins until the top is bubbling and lightly browned."},
		{"degrees untouched", "Preheat the oven to 350 degrees.", 2, "Preheat the oven to 350 degrees."},
		{"seconds untouched", "Microwave the butter for about 40 seconds to just barely melt it.", 2,
			"Microwave the butter for about 40 seconds to just barely melt it."},
		{"minute range untouched", "Bake for 9-11 minutes until the cookies look puffy.", 2,
			"Bake for 9-11 minutes until the cookies look puffy."},
		{"simmer time untouched", "Turn down to low and simmer for 20 minutes.", 2, "Turn down to low and simmer for 20 minutes."},

		// bare counts in prose stay fixed
		{"batches untouched", "cook the mince in two batches for about 10 mins until browned all over.", 2,
			"cook the mince in two batches for about 10 mins until browned all over."},
		{"ball count untouched", "Roll the dough into 12 large balls (or 9 for HUGELY awesome cookies) and place on a cookie sheet.", 2,
			"Roll the dough into 12 large balls (or 9 for HUGELY awesome cookies) and place on a cookie sheet."},

		// quantities with units do scale
		{"grams in step scale", "cook 940 g lean beef mince until browned.", 2, "cook 1.9 kg lean beef mince until browned."},
		{"slices scale, pack size fixed", "Finely chop 5 slices of prosciutto from a 110 g pack, then stir through.", 1.2,
			"Finely chop 6 slices of prosciutto from a 110 g pack, then stir through."},
		{"sauce grams scale", "Drizzle over roughly 160 g ready-made or homemade white sauce.", 0.5,
			"Drizzle over roughly 80 g ready-made or homemade white sauce."},

		// serving references track the ratio
		{"serves updates", "Garnish and serve. Serves 5.", 8.0 / 5, "Garnish and serve. Serves 8."},
		{"makes updates", "Makes 12 cookies.", 2, "Makes 24 cookies."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScaleInstruction(tt.in, tt.ratio); got != tt.want {
				t.Errorf("ScaleInstruction(%q, %v)\n got:  %q\n want: %q", tt.in, tt.ratio, got, tt.want)
			}
		})
	}
}
