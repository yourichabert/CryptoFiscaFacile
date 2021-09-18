package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/davecgh/go-spew/spew"
	"github.com/fiscafacile/CryptoFiscaFacile/wallet"
	"github.com/shopspring/decimal"
)

type Cession struct {
	Source                               string
	Note                                 string
	Date211                              time.Time
	ValeurPortefeuille212                decimal.Decimal
	Prix213                              decimal.Decimal
	Frais214                             decimal.Decimal
	PrixNetDeFrais215                    decimal.Decimal
	SoulteRecueOuVersee216               decimal.Decimal
	PrixNetDeSoulte217                   decimal.Decimal
	PrixNet218                           decimal.Decimal
	PrixTotalAcquisition220              decimal.Decimal
	FractionDeCapital221                 decimal.Decimal
	SoulteRecueEnCasDechangeAnterieur222 decimal.Decimal
	PrixTotalAcquisitionNet223           decimal.Decimal
	PlusMoinsValue                       decimal.Decimal
}

type Cessions []Cession

func (c *Cession) Calculate() {
	c.Prix213 = c.PrixNetDeFrais215.Add(c.Frais214)
	c.PrixNetDeSoulte217 = c.Prix213.Sub(c.SoulteRecueOuVersee216)
	// c.PrixNetDeSoulte217 = c.Prix213.Add(c.SoulteRecueOuVersee216)
	c.PrixNet218 = c.Prix213.Sub(c.Frais214).Sub(c.SoulteRecueOuVersee216)
	// c.PrixNet218 = c.Prix213.Sub(c.Frais214).Add(c.SoulteRecueOuVersee216)
	c.PrixTotalAcquisitionNet223 = c.PrixTotalAcquisition220.Sub(c.FractionDeCapital221).Sub(c.SoulteRecueEnCasDechangeAnterieur222)
	if !c.ValeurPortefeuille212.IsZero() {
		c.PlusMoinsValue = c.PrixNet218.Sub(c.PrixTotalAcquisitionNet223.Mul(c.PrixNetDeSoulte217).Div(c.ValeurPortefeuille212))
	}
}

func (c Cession) Println() {
	fmt.Println("211 Date de la cession :", c.Date211.Format("02-01-2006"))
	fmt.Println("212 Valeur globale du portefeuille au moment de la cession :", c.ValeurPortefeuille212.RoundBank(0))
	fmt.Println("213 Prix de cession :", c.Prix213.RoundBank(0))
	fmt.Println("214 Frais de cession :", c.Frais214.RoundBank(0))
	fmt.Println("215 Prix de cession net des frais :", c.PrixNetDeFrais215.RoundBank(0))
	fmt.Println("216 Soulte reçue ou versée lors de la cession :", c.SoulteRecueOuVersee216.RoundBank(0))
	fmt.Println("217 Prix de cession net des soultes :", c.PrixNetDeSoulte217.RoundBank(0))
	fmt.Println("218 Prix de cession net des frais et soultes :", c.PrixNet218.RoundBank(0))
	fmt.Println("220 Prix total d’acquisition :", c.PrixTotalAcquisition220.RoundBank(0))
	fmt.Println("221 Fractions de capital initial contenues dans le prix total d’acquisition :", c.FractionDeCapital221.RoundBank(0))
	fmt.Println("222 Soultes reçues en cas d’échanges antérieurs à la cession :", c.SoulteRecueEnCasDechangeAnterieur222.RoundBank(0))
	fmt.Println("223 Prix total d’acquisition net :", c.PrixTotalAcquisitionNet223.RoundBank(0))
	fmt.Println("Plus-values et moins-values :", c.PlusMoinsValue.RoundBank(0))
}

type ValuedTX struct {
	ValeurPEPS  decimal.Decimal
	QtyToFind   decimal.Decimal
	QtyIn       decimal.Decimal
	QtyOut      decimal.Decimal
	NativeValue decimal.Decimal
	TX          wallet.TX
}

type BuyingPrice struct {
	TransactionsValiorisees []ValuedTX
	Montant                 decimal.Decimal
}

type TotalBuyingPrice struct {
	Acquisitions         map[string]BuyingPrice
	PrixTotalAcquisition decimal.Decimal
}

func (pta *TotalBuyingPrice) CalculateFIFO(global wallet.TXsByCategory, native string, loc *time.Location) (err error) {
	if pta.Acquisitions == nil {
		pta.Acquisitions = make(map[string]BuyingPrice)
	}
	// source Bofip
	// RPPM - Plus-values sur biens meubles et taxe forfaitaire sur les objets précieux
	// - Cession d'actifs numériques à titre occasionnel - Base d'imposition
	// Cas des cessions antérieures au 1er Janvier 2019
	// 130
	// L'article 41 de la loi n° 2018-1317 du 28 décembre 2018 de finances pour 2019,
	// codifié à l'article 150 VH bis du CGI, s'appliquant aux cessions réalisées à
	// compter du 1er janvier 2019, il convient, pour la détermination du prix total
	// d'acquisition, de n'inclure dans ce dernier que les prix effectifs d'acquisition
	// des actifs détenus à cette date.
	// Ainsi, en cas de cessions réalisées antérieurement au 1er janvier 2019, il
	// convient notamment de ne pas inclure dans le prix total d'acquisition déclaré à
	// l'occasion de la première cession réalisée postérieurement à cette date, les
	// prix d'acquisition :
	// - mentionnés dans les déclarations de plus-values de cessions déclarées en
	//   application du droit en vigueur avant le 1er janvier 2019 ;
	// - n'ayant pas été déclarés, conformément au droit en vigueur avant le 1er
	//   janvier 2019 (cessions dont le prix de cession était inférieur à 5 000 € et
	//   ayant bénéficié de l'exonération prévue au 2° du II de l'article 150 UA du CGI
	//   par exemple) ;
	// - n'ayant pas été déclarés en contravention avec le droit en vigueur avant le
	//   1er janvier 2019.
	// Il est rappelé que les éventuelles plus-values réalisées antérieurement au
	// 1er janvier 2019 relèvent du droit de reprise de l'administration.
	// Remarque : En cas d'échange entre actifs numériques réalisé, même sans soulte,
	// antérieurement au 1er janvier 2019, le prix total d'acquisition à retenir à
	// compter du 1er janvier 2019 est constitué de la valeur de l'actif numérique
	// remis lors de cet échange (valeur à la date de cet échange). Corrélativement,
	// le prix d'acquisition retenu à l'occasion de cette cession n'est pas inclus dans
	// le prix total d'acquisition déclaré à compter du 1er janvier 2019.
	// Par ailleurs, les moins-values constatées lors des cessions réalisées
	// antérieurement au 1er janvier 2019 ne peuvent être imputées sur d'éventuelles
	// plus-values réalisées, quelle que soit leur date de réalisation.
	date2019Jan1 := time.Date(2019, time.January, 1, 0, 0, 0, 0, loc)
	globalWallet2019Jan1 := global.GetWallets(date2019Jan1, false, true)
	// globalWallet2019Jan1.Println("2019 Jan 1st Global", "")
	// Consolidate all knowns TXs
	var allTXs wallet.TXs
	for k := range global {
		// if k != "Transfers" { // Transfers are not compatible with CUS
		allTXs = append(allTXs, global[k]...)
		// }
	}
	allTXs.SortByDate(false)
	for crypto, quantity := range globalWallet2019Jan1.Currencies {
		if quantity.IsNegative() {
			globalWallet2019Jan1.Println("2019 Jan 1st Global", "")
			return errors.New("Erreur : votre stock initial de " + crypto + " au 1 Janvier 2019 a un montant négatif, il doit manquer des transactions !")
		}
		amountToFind := quantity
		var fifoValue decimal.Decimal
		var valuedTXs []ValuedTX
		for _, tx := range allTXs {
			// Find all Tx before 2019 Jan 1st ...
			if tx.Timestamp.Before(date2019Jan1) {
				// ... that have the wanted crypto into Items["To"]
				vtx := ValuedTX{
					QtyToFind: amountToFind,
					TX:        tx,
				}
				for _, c := range tx.Items["To"] {
					if c.Code == crypto {
						if amountToFind.LessThan(c.Amount) {
							vtx.QtyIn = vtx.QtyIn.Add(amountToFind)
						} else {
							vtx.QtyIn = vtx.QtyIn.Add(c.Amount)
						}
						amountToFind = amountToFind.Sub(c.Amount)
					}
				}
				// ... and the ones consumpting the wanted crypto
				for _, c := range tx.Items["From"] {
					if c.Code == crypto {
						vtx.QtyOut = vtx.QtyOut.Add(c.Amount)
						amountToFind = amountToFind.Add(c.Amount)
					}
				}
				for _, c := range tx.Items["Fee"] {
					if c.Code == crypto {
						vtx.QtyOut = vtx.QtyOut.Add(c.Amount)
						amountToFind = amountToFind.Add(c.Amount)
					}
				}
				vtx.NativeValue = vtx.QtyIn.Sub(vtx.QtyOut)
				if !vtx.NativeValue.IsZero() {
					c := wallet.Currency{Code: crypto}
					rate, err := c.GetExchangeRate(tx.Timestamp, native)
					if err != nil {
						// Allow to look for rate on the next day as for Forks, no rate available on fork day !
						rate, err = c.GetExchangeRate(tx.Timestamp.Add(24*time.Hour), native)
						if err != nil {
							log.Println(err)
						}
					}
					if err == nil {
						fmt.Print(".")
						vtx.NativeValue = vtx.NativeValue.Mul(rate)
						fifoValue = fifoValue.Add(vtx.NativeValue)
						vtx.ValeurPEPS = fifoValue
						valuedTXs = append(valuedTXs, vtx)
					}
				}
				if !amountToFind.IsPositive() {
					pta.Acquisitions[crypto] = BuyingPrice{
						TransactionsValiorisees: valuedTXs,
						Montant:                 fifoValue,
					}
					pta.PrixTotalAcquisition = pta.PrixTotalAcquisition.Add(fifoValue)
					break
				}
			}
		}
		if amountToFind.IsPositive() {
			return errors.New("Impossible de trouver assez de transacations pour calculer le prix d'acquisition de " + crypto + " par PEPS, il en manque " + amountToFind.String() + " !")
		}
	}
	return nil
}

type Cerfa2086 struct {
	cs                Cessions
	pta               TotalBuyingPrice
	cashInBNC         map[int]bool
	airdrops          map[int]wallet.WalletCurrencies
	commercialRebates map[int]wallet.WalletCurrencies
	interests         map[int]wallet.WalletCurrencies
	minings           map[int]wallet.WalletCurrencies
	referrals         map[int]wallet.WalletCurrencies
}

func New2086() Cerfa2086 {
	var c Cerfa2086
	c.cashInBNC = make(map[int]bool)
	c.airdrops = make(map[int]wallet.WalletCurrencies)
	c.commercialRebates = make(map[int]wallet.WalletCurrencies)
	c.interests = make(map[int]wallet.WalletCurrencies)
	c.minings = make(map[int]wallet.WalletCurrencies)
	c.referrals = make(map[int]wallet.WalletCurrencies)
	return c
}

func (c2086 *Cerfa2086) CalculatePVMV(global wallet.TXsByCategory, native string, loc *time.Location) (err error) {
	// Calculate initial PTA
	err = c2086.pta.CalculateFIFO(global, native, loc)
	if err != nil {
		return err
	}
	jan1st2019 := time.Date(2019, time.January, 1, 0, 0, 0, 0, loc)
	jan1st2020 := time.Date(2020, time.January, 1, 0, 0, 0, 0, loc)
	jan1st2021 := time.Date(2021, time.January, 1, 0, 0, 0, 0, loc)
	jan1st2022 := time.Date(2022, time.January, 1, 0, 0, 0, 0, loc)
	// Consolidate all CashIn/CashOut TXs
	var cashInOut wallet.TXs
	cashInOut = append(cashInOut, global["CashIn"].After(jan1st2019)...)
	cashInOut = append(cashInOut, global["CashOut"].After(jan1st2019)...)
	var airdrops wallet.TXs
	airdrops = append(airdrops, global["AirDrops"].After(jan1st2019).AddFromNativeValue(native)...)
	c2086.airdrops[2019] = airdrops.Before(jan1st2020).GetBalances(true, false)
	c2086.airdrops[2020] = airdrops.After(jan1st2020).Before(jan1st2021).GetBalances(true, false)
	c2086.airdrops[2021] = airdrops.After(jan1st2021).Before(jan1st2022).GetBalances(true, false)
	cashInOut = append(cashInOut, airdrops...)
	var commercialRebates wallet.TXs
	commercialRebates = append(commercialRebates, global["CommercialRebates"].After(jan1st2019).AddFromNativeValue(native)...)
	c2086.commercialRebates[2019] = commercialRebates.Before(jan1st2020).GetBalances(true, false)
	c2086.commercialRebates[2020] = commercialRebates.After(jan1st2020).Before(jan1st2021).GetBalances(true, false)
	c2086.commercialRebates[2021] = commercialRebates.After(jan1st2021).Before(jan1st2022).GetBalances(true, false)
	cashInOut = append(cashInOut, commercialRebates...)
	cashInOut = append(cashInOut, global["Gifts"].After(jan1st2019).AddFromNativeValue(native)...)
	var referrals wallet.TXs
	referrals = append(referrals, global["Referrals"].After(jan1st2019).AddFromNativeValue(native)...)
	c2086.referrals[2019] = referrals.Before(jan1st2020).GetBalances(true, false)
	c2086.referrals[2020] = referrals.After(jan1st2020).Before(jan1st2021).GetBalances(true, false)
	c2086.referrals[2021] = referrals.After(jan1st2021).Before(jan1st2022).GetBalances(true, false)
	cashInOut = append(cashInOut, referrals...)
	if c2086.cashInBNC[2019] {
		fmt.Print("Conversion des Interests/Minings en CashIn pour les transactions de 2019...")
		var interests wallet.TXs
		interests = append(interests, global["Interests"].After(jan1st2019).Before(jan1st2020).AddFromNativeValue(native)...)
		c2086.interests[2019] = interests.GetBalances(true, false)
		cashInOut = append(cashInOut, interests...)
		var minings wallet.TXs
		minings = append(minings, global["Minings"].After(jan1st2019).Before(jan1st2020).AddFromNativeValue(native)...)
		c2086.minings[2019] = minings.GetBalances(true, false)
		cashInOut = append(cashInOut, minings...)
	}
	if c2086.cashInBNC[2020] {
		fmt.Print("Conversion des Interests/Minings en CashIn pour les transactions de 2020...")
		var interests wallet.TXs
		interests = append(interests, global["Interests"].After(jan1st2020).Before(jan1st2021).AddFromNativeValue(native)...)
		c2086.interests[2020] = interests.GetBalances(true, false)
		cashInOut = append(cashInOut, interests...)
		var minings wallet.TXs
		minings = append(minings, global["Minings"].After(jan1st2020).Before(jan1st2021).AddFromNativeValue(native)...)
		c2086.minings[2020] = minings.GetBalances(true, false)
		cashInOut = append(cashInOut, minings...)
	}
	if c2086.cashInBNC[2021] {
		fmt.Print("Conversion des Interests/Minings en CashIn pour les transactions de 2021...")
		var interests wallet.TXs
		interests = append(interests, global["Interests"].After(jan1st2021).Before(jan1st2022).AddFromNativeValue(native)...)
		c2086.interests[2021] = interests.GetBalances(true, false)
		cashInOut = append(cashInOut, interests...)
		var minings wallet.TXs
		minings = append(minings, global["Minings"].After(jan1st2021).Before(jan1st2022).AddFromNativeValue(native)...)
		c2086.minings[2021] = minings.GetBalances(true, false)
		cashInOut = append(cashInOut, minings...)
	}
	cashInOut.SortByDate(true)
	// Calculate PV starting on 2019 Jan 1st
	var fractionCapital decimal.Decimal
	for _, tx := range cashInOut {
		if len(tx.Items["To"]) > 0 {
			for _, to := range tx.Items["To"] {
				if to.IsFiat() && to.Amount.GreaterThanOrEqual(decimal.NewFromInt(1)) { // CashOut
					c := Cession{Date211: tx.Timestamp}
					infos := strings.SplitN(tx.Note, ":", 2)
					c.Source = infos[0]
					c.Note = infos[1]
					// Valeur globale du portefeuille au moment de la cession
					// Il s’agit de la somme des valeurs, évaluées au moment de la cession
					// imposable, des différents actifs numériques et droits s'y rapportant,
					// détenus par le cédant avant de procéder à la cession, quel que soit
					// leur support de conservation (plateformes d’échanges, y compris
					// étrangères, serveurs personnels, dispositif de stockage hors-ligne,
					// etc.). Cette valorisation doit s’effectuer au moment de chaque cession
					// imposable en application de l’article 150 VH bis du CGI.
					globalWallet := global.GetWallets(tx.Timestamp, false, true)
					globalWalletTotalValue, err := globalWallet.CalculateTotalValue(native)
					if err != nil {
						log.Println("Error Calculating Global Wallet at", tx.Timestamp, err)
					}
					// spew.Dump(globalWallet)
					c.ValeurPortefeuille212 = globalWalletTotalValue.Amount
					// Prix de cession
					// Il correspond au prix réel perçu ou à la valeur de la contrepartie
					// obtenue par le cédant lors de la cession.
					if to.Code == native {
						c.PrixNetDeFrais215 = to.Amount
					} else {
						rate, err := to.GetExchangeRate(tx.Timestamp, native)
						if err == nil {
							c.PrixNetDeFrais215 = to.Amount.Mul(rate)
						} else {
							log.Println("Rate missing : CashOut integration into Prix213", spew.Sdump(tx, c))
						}
					}
					// Prix de cession - Frais
					// Il est réduit, sur justificatifs, des frais supportés par le cédant à
					// l’occasion de cette cession. Ces frais s'entendent, notamment, de
					// ceux perçus à l’occasion de l’opération imposable par les plateformes
					// où s'effectuent les cessions à titre onéreux d'actifs numériques ou
					// de droits s'y rapportant ainsi que de ceux perçus par les membres du
					// réseau (appelés "mineurs") chargés de vérifier et valider les
					// transactions qui s'y opèrent. Le paiement de ces frais de transaction
					// perçus par les plateformes ou les "mineurs" peut s'effectuer au moyen
					// d'actifs numériques. Or, dans ce cas, ce paiement est la contrepartie
					// d'un service fourni au cédant et constitue une opération imposable au
					// sens du I de l'article 150 VH bis du CGI. A titre de mesure de
					// simplification, il est toutefois admis que la cession en tant que
					// telle et les différentes prestations de services rendues en
					// contrepartie des frais perçus par les plateformes et les "mineurs"
					// soient assimilées à une seule et même opération de cession pour
					// l'application de l'article 150 VH bis du CGI, pour laquelle le
					// contribuable détermine une seule plus ou moins-value, en déduisant
					// ces frais du prix de cession.
					for _, f := range tx.Items["Fee"] {
						if f.Code == native {
							c.Frais214 = c.Frais214.Add(f.Amount)
						} else {
							rate, err := f.GetExchangeRate(tx.Timestamp, native)
							if err == nil {
								c.Frais214 = c.Frais214.Add(f.Amount.Mul(rate))
							} else {
								log.Println("Rate missing : CashOut integration into Frais214", spew.Sdump(tx, c))
							}
						}
					}
					// Prix de cession - Soultes
					// Le prix de cession doit être majoré de la soulte que le cédant a
					// reçue lors de la cession ou minoré de la soulte qu’il a versée lors
					// de cette même cession.
					// c.SoulteRecueOuVersee216 = ???
					c.PrixTotalAcquisition220 = c2086.pta.PrixTotalAcquisition
					// Fractions de capital initial
					// Il s’agit de la fraction de capital contenue dans la valeur ou le
					// prix de chacune des différentes cessions d'actifs numériques à titre
					// gratuit ou onéreux réalisées antérieurement, hors opérations d’échange
					// ayant bénéficié du sursis d’imposition sans soulte.
					c.FractionDeCapital221 = fractionCapital
					// Soulte reçue en cas d’échanges antérieurs à la cession
					// Lorsqu’un ou plusieurs échanges avec soulte reçue par le cédant ont été
					// réalisés antérieurement à la cession imposable, le prix total d’acquisition
					// est minoré du montant des soultes. Indiquez donc les montants reçus.
					// c.SoulteRecueEnCasDechangeAnterieur222 = ???
					c.Calculate() // to have 217 and 223
					c2086.cs = append(c2086.cs, c)
					// Les frais déductibles, quels qu'ils soient, ne viennent pas en
					// diminution du prix de cession pour la détermination du quotient du
					// prix de cession sur la valeur globale du portefeuille (ils doivent
					// seulement être déduits du prix de cession qui constitue le premier
					// terme de la différence prévue dans la formule de calcul mentionnée
					// ci-dessus).
					var coefCession decimal.Decimal
					if !c.ValeurPortefeuille212.IsZero() {
						coefCession = c.PrixNetDeSoulte217.Div(c.ValeurPortefeuille212)
					} else {
						log.Println("Erreur : Valeur du PF nul après", spew.Sdump(tx))
					}
					fractionAcquisition := coefCession.Mul(c.PrixTotalAcquisitionNet223)
					fractionCapital = fractionCapital.Add(fractionAcquisition)
				}
			}
		}
		if len(tx.Items["From"]) > 0 {
			for _, from := range tx.Items["From"] {
				if from.IsFiat() { // CashIn
					// Prix total d’acquisition du portefeuille
					// Le prix total d'acquisition du portefeuille d'actifs numériques est
					// égal à la somme de tous les prix acquittés en monnaie ayant cours
					// légal à l'occasion de l'ensemble des acquisitions d’actifs numériques
					// (sauf opérations d'échange ayant bénéficié du sursis d'imposition)
					// réalisées avant la cession, et de la valeur des biens ou services,
					// comprenant le cas échéant les soultes versées, fournis en
					// contrepartie de ces acquisitions.
					if from.Code == native {
						c2086.pta.PrixTotalAcquisition = c2086.pta.PrixTotalAcquisition.Add(from.Amount)
					} else {
						rate, err := from.GetExchangeRate(tx.Timestamp, native)
						if err == nil {
							c2086.pta.PrixTotalAcquisition = c2086.pta.PrixTotalAcquisition.Add(rate.Mul(from.Amount))
						} else {
							log.Println("Rate missing during CashIn integration into PrixTotalAcquisition", spew.Sdump(tx))
						}
					}
				}
			}
		} else {
			log.Println("Malformated TX :", tx)
		}
	}
	return
}

func (c2086 Cerfa2086) Println(native string) {
	for year := 2019; year <= time.Now().Year(); year++ {
		var plusMoinsValueGlobale decimal.Decimal
		fmt.Println("-------------------------")
		fmt.Println("| Cerfa 2086 année " + strconv.Itoa(year) + " |")
		fmt.Println("-------------------------")
		for _, c := range c2086.cs {
			if c.Date211.After(time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)) {
				if c.Date211.Before(time.Date(year, time.December, 31, 23, 59, 59, 999, time.UTC)) {
					c.Println()
					fmt.Println("-------------------------")
					plusMoinsValueGlobale = plusMoinsValueGlobale.Add(c.PlusMoinsValue)
				} else {
					break
				}
			}
		}
		fmt.Println("224 Plus-value ou moins-value globale :", plusMoinsValueGlobale.RoundBank(0))
		fmt.Println("Voici votre récapitulatif par catégorie de l'année fiscale " + strconv.Itoa(year) + " :")
		fmt.Println("- Airdrops fortuits : " + c2086.airdrops[year][native].Neg().RoundBank(2).String() + " " + native)
		fmt.Println("- Remises commerciales (cashback, etc) : " + c2086.commercialRebates[year][native].Neg().RoundBank(2).String() + " " + native)
		fmt.Println("- Intérêts (lending, etc) : " + c2086.interests[year][native].Neg().RoundBank(2).String() + " " + native)
		fmt.Println("- Revenus de récompenses (staking, mining, aidrops avec contrepartie, etc) : " + c2086.minings[year][native].Neg().RoundBank(2).String() + " " + native)
		fmt.Println("- Revenus de parrainage : " + c2086.referrals[year][native].Neg().RoundBank(2).String() + " " + native)
		if c2086.cashInBNC[year] {
			fmt.Println("Voici donc vos obligations déclaratives :")
			fmt.Println("- case 5KU du formulaire 2042-C-PRO : " + c2086.referrals[year][native].Add(c2086.minings[year][native]).Neg().RoundBank(0).String() + " " + native + " (parrainage + récompenses)")
			fmt.Println("- case 2TR du formulaire 2047 et à reporter sur la déclaration principale : " + c2086.interests[year][native].Neg().RoundBank(0).String() + " " + native + " (intérêts)")
		}
		fmt.Println("Pour rappel, vous avez un total de " + c2086.airdrops[year][native].Add(c2086.commercialRebates[year][native]).Neg().RoundBank(0).String() + " " + native + " non imposable (airdrops fortuits + remises commerciales).")
		fmt.Println("-------------------------")
	}
}

func (c2086 Cerfa2086) ToXlsx(filename, native string) {
	f := excelize.NewFile()
	if len(c2086.pta.Acquisitions) > 0 {
		sheet := "Prix Total Acquisition PEPS"
		f.NewSheet(sheet)
		f.SetCellValue(sheet, "A1", "Date")
		f.SetCellValue(sheet, "B1", "Crypto")
		f.SetCellValue(sheet, "C1", "Quantité à Trouver")
		f.SetCellValue(sheet, "D1", "Quantité Entrée")
		f.SetCellValue(sheet, "E1", "Quantité Sortie")
		f.SetCellValue(sheet, "F1", "Valeur "+native)
		f.SetCellValue(sheet, "G1", "Note")
		row := 2
		for crypto, buyPrice := range c2086.pta.Acquisitions {
			for _, vtx := range buyPrice.TransactionsValiorisees {
				f.SetCellValue(sheet, "A"+strconv.Itoa(row), vtx.TX.Timestamp.Format("02/01/2006 15:04:05"))
				f.SetCellValue(sheet, "B"+strconv.Itoa(row), crypto)
				toFind, _ := vtx.QtyToFind.Float64()
				f.SetCellValue(sheet, "C"+strconv.Itoa(row), toFind)
				if !vtx.QtyIn.IsZero() {
					in, _ := vtx.QtyIn.Float64()
					f.SetCellValue(sheet, "D"+strconv.Itoa(row), in)
				}
				if !vtx.QtyOut.IsZero() {
					out, _ := vtx.QtyOut.Float64()
					f.SetCellValue(sheet, "E"+strconv.Itoa(row), out)
				}
				val, _ := vtx.NativeValue.RoundBank(2).Float64()
				f.SetCellValue(sheet, "F"+strconv.Itoa(row), val)
				f.SetCellValue(sheet, "G"+strconv.Itoa(row), vtx.TX.Note)
				// vtx.ValeurPEPS
				row += 1
			}
			// buyPrice.Montant
		}
		f.SetColWidth(sheet, "A", "A", 19)
		f.SetColWidth(sheet, "C", "C", 17)
		f.SetColWidth(sheet, "D", "E", 15)
		f.SetColWidth(sheet, "G", "G", 50)
	}
	// c2086.pta.PrixTotalAcquisition
	for year := 2019; year <= time.Now().Year(); year++ {
		sheet := strconv.Itoa(year)
		f.NewSheet(sheet)
		f.SetCellValue(sheet, "A2", 211)
		f.SetCellValue(sheet, "A3", 212)
		f.SetCellValue(sheet, "A4", 213)
		f.SetCellValue(sheet, "A5", 214)
		f.SetCellValue(sheet, "A6", 215)
		f.SetCellValue(sheet, "A7", 216)
		f.SetCellValue(sheet, "A8", 217)
		f.SetCellValue(sheet, "A9", 218)
		f.SetCellValue(sheet, "A10", 220)
		f.SetCellValue(sheet, "A11", 221)
		f.SetCellValue(sheet, "A12", 222)
		f.SetCellValue(sheet, "A13", 223)
		f.SetCellValue(sheet, "A16", 224)
		f.SetCellValue(sheet, "B1", "Cession")
		f.SetCellValue(sheet, "B2", "Date de la cession")
		f.SetCellValue(sheet, "B3", "Valeur globale du portefeuille au moment de la cession")
		f.SetCellValue(sheet, "B4", "Prix de cession")
		f.SetCellValue(sheet, "B5", "Frais de cession")
		f.SetCellValue(sheet, "B6", "Prix de cession net des frais")
		f.SetCellValue(sheet, "B7", "Soulte reçue ou versée lors de la cession")
		f.SetCellValue(sheet, "B8", "Prix de cession net des soultes")
		f.SetCellValue(sheet, "B9", "Prix de cession net des frais et soultes")
		f.SetCellValue(sheet, "B10", "Prix total d’acquisition")
		f.SetCellValue(sheet, "B11", "Fractions de capital initial contenues dans le prix total d’acquisition")
		f.SetCellValue(sheet, "B12", "Soultes reçues en cas d’échanges antérieurs à la cession")
		f.SetCellValue(sheet, "B13", "Prix total d’acquisition net")
		f.SetCellValue(sheet, "B14", "Plus-values et moins-values")
		f.SetCellValue(sheet, "B16", "Plus-value ou moins-value globale")
		f.SetColWidth(sheet, "B", "B", 60)
		var plusMoinsValueGlobale decimal.Decimal
		col := "C"
		count := 1
		for _, c := range c2086.cs {
			if c.Date211.After(time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)) {
				if c.Date211.Before(time.Date(year, time.December, 31, 23, 59, 59, 999, time.UTC)) {
					f.SetCellValue(sheet, col+"1", "#"+strconv.Itoa(count))
					f.AddComment(sheet, col+"1", `{"author":"`+c.Source+`: ","text":"`+c.Note+`"}`)
					f.SetCellValue(sheet, col+"2", c.Date211.Format("02/01/2006"))
					f.SetCellValue(sheet, col+"3", c.ValeurPortefeuille212.RoundBank(0).IntPart())
					f.SetCellValue(sheet, col+"4", c.Prix213.RoundBank(0).IntPart())
					f.SetCellValue(sheet, col+"5", c.Frais214.RoundBank(0).IntPart())
					f.SetCellValue(sheet, col+"6", c.PrixNetDeFrais215.RoundBank(0).IntPart())
					f.SetCellValue(sheet, col+"7", c.SoulteRecueOuVersee216.RoundBank(0).IntPart())
					f.SetCellValue(sheet, col+"8", c.PrixNetDeSoulte217.RoundBank(0).IntPart())
					f.SetCellValue(sheet, col+"9", c.PrixNet218.RoundBank(0).IntPart())
					f.SetCellValue(sheet, col+"10", c.PrixTotalAcquisition220.RoundBank(0).IntPart())
					f.SetCellValue(sheet, col+"11", c.FractionDeCapital221.RoundBank(0).IntPart())
					f.SetCellValue(sheet, col+"12", c.SoulteRecueEnCasDechangeAnterieur222.RoundBank(0).IntPart())
					f.SetCellValue(sheet, col+"13", c.PrixTotalAcquisitionNet223.RoundBank(0).IntPart())
					f.SetCellValue(sheet, col+"14", c.PlusMoinsValue.RoundBank(0).IntPart())
					plusMoinsValueGlobale = plusMoinsValueGlobale.Add(c.PlusMoinsValue)
					count += 1
					num := count + 2
					col = ""
					for num > 0 {
						col = string(rune((num-1)%26+65)) + col
						num = (num - 1) / 26
					}
				} else {
					break
				}
			}
		}
		f.SetCellValue(sheet, "C16", plusMoinsValueGlobale.RoundBank(0).IntPart())
		f.SetCellValue(sheet, "A18", "Voici votre récapitulatif par catégorie de l'année fiscale "+sheet+" :")
		f.SetCellValue(sheet, "A19", "- Airdrops fortuits : "+c2086.airdrops[year][native].Neg().RoundBank(2).String()+" "+native)
		f.SetCellValue(sheet, "A20", "- Remises commerciales (cashback, etc) : "+c2086.commercialRebates[year][native].Neg().RoundBank(2).String()+" "+native)
		f.SetCellValue(sheet, "A21", "- Intérêts (lending, etc) : "+c2086.interests[year][native].Neg().RoundBank(2).String()+" "+native)
		f.SetCellValue(sheet, "A22", "- Revenus de récompenses (staking, mining, aidrops avec contrepartie, etc) : "+c2086.minings[year][native].Neg().RoundBank(2).String()+" "+native)
		f.SetCellValue(sheet, "A23", "- Revenus de parrainage : "+c2086.referrals[year][native].Neg().RoundBank(2).String()+" "+native)
		next := "A25"
		if c2086.cashInBNC[year] {
			f.SetCellValue(sheet, "A25", "Voici donc vos obligations déclaratives :")
			f.SetCellValue(sheet, "A26", "- case 5KU du formulaire 2042-C-PRO : "+c2086.referrals[year][native].Add(c2086.minings[year][native]).Neg().RoundBank(0).String()+" "+native+" (parrainage + récompenses)")
			f.SetCellValue(sheet, "A27", "- case 2TR du formulaire 2047 et à reporter sur la déclaration principale : "+c2086.interests[year][native].Neg().RoundBank(0).String()+" "+native+" (intérêts)")
			next = "A29"
		}
		f.SetCellValue(sheet, next, "Pour rappel, vous avez un total de "+c2086.airdrops[year][native].Add(c2086.commercialRebates[year][native]).Neg().RoundBank(0).String()+" "+native+" non imposable (airdrops fortuits + remises commerciales).")
	}
	f.DeleteSheet("Sheet1")
	if err := f.SaveAs(filename); err != nil {
		log.Fatal(err)
	}
}
