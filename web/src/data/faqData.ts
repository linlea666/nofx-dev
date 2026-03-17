import type { LucideIcon } from 'lucide-react'
import {
  Rocket,
  Download,
  Settings,
  TrendingUp,
  Wrench,
  Shield,
  Star,
  Cpu,
  Heart,
} from 'lucide-react'

export interface FAQItem {
  id: string
  questionKey: string
  answerKey: string
}

export interface FAQCategory {
  id: string
  titleKey: string
  icon: LucideIcon
  items: FAQItem[]
}

export const faqCategories: FAQCategory[] = [
  {
    id: 'getting-started',
    titleKey: 'faqCategoryGettingStarted',
    icon: Rocket,
    items: [
      { id: 'what-is-nofx', questionKey: 'faqWhatIsNOFX', answerKey: 'faqWhatIsNOFXAnswer' },
      { id: 'how-does-it-work', questionKey: 'faqHowDoesItWork', answerKey: 'faqHowDoesItWorkAnswer' },
      { id: 'is-profitable', questionKey: 'faqIsProfitable', answerKey: 'faqIsProfitableAnswer' },
      { id: 'supported-exchanges', questionKey: 'faqSupportedExchanges', answerKey: 'faqSupportedExchangesAnswer' },
      { id: 'supported-ai-models', questionKey: 'faqSupportedAIModels', answerKey: 'faqSupportedAIModelsAnswer' },
      { id: 'system-requirements', questionKey: 'faqSystemRequirements', answerKey: 'faqSystemRequirementsAnswer' },
    ],
  },
  {
    id: 'installation',
    titleKey: 'faqCategoryInstallation',
    icon: Download,
    items: [
      { id: 'how-to-install', questionKey: 'faqHowToInstall', answerKey: 'faqHowToInstallAnswer' },
      { id: 'windows-installation', questionKey: 'faqWindowsInstallation', answerKey: 'faqWindowsInstallationAnswer' },
      { id: 'docker-deployment', questionKey: 'faqDockerDeployment', answerKey: 'faqDockerDeploymentAnswer' },
      { id: 'manual-installation', questionKey: 'faqManualInstallation', answerKey: 'faqManualInstallationAnswer' },
      { id: 'server-deployment', questionKey: 'faqServerDeployment', answerKey: 'faqServerDeploymentAnswer' },
      { id: 'update-nofx', questionKey: 'faqUpdateNOFX', answerKey: 'faqUpdateNOFXAnswer' },
    ],
  },
  {
    id: 'configuration',
    titleKey: 'faqCategoryConfiguration',
    icon: Settings,
    items: [
      { id: 'configure-ai-models', questionKey: 'faqConfigureAIModels', answerKey: 'faqConfigureAIModelsAnswer' },
      { id: 'configure-exchanges', questionKey: 'faqConfigureExchanges', answerKey: 'faqConfigureExchangesAnswer' },
      { id: 'binance-api-setup', questionKey: 'faqBinanceAPISetup', answerKey: 'faqBinanceAPISetupAnswer' },
      { id: 'hyperliquid-setup', questionKey: 'faqHyperliquidSetup', answerKey: 'faqHyperliquidSetupAnswer' },
      { id: 'create-strategy', questionKey: 'faqCreateStrategy', answerKey: 'faqCreateStrategyAnswer' },
      { id: 'create-trader', questionKey: 'faqCreateTrader', answerKey: 'faqCreateTraderAnswer' },
    ],
  },
  {
    id: 'trading',
    titleKey: 'faqCategoryTrading',
    icon: TrendingUp,
    items: [
      { id: 'how-ai-decides', questionKey: 'faqHowAIDecides', answerKey: 'faqHowAIDecidesAnswer' },
      { id: 'decision-frequency', questionKey: 'faqDecisionFrequency', answerKey: 'faqDecisionFrequencyAnswer' },
      { id: 'no-trades-executing', questionKey: 'faqNoTradesExecuting', answerKey: 'faqNoTradesExecutingAnswer' },
      { id: 'only-short-positions', questionKey: 'faqOnlyShortPositions', answerKey: 'faqOnlyShortPositionsAnswer' },
      { id: 'leverage-settings', questionKey: 'faqLeverageSettings', answerKey: 'faqLeverageSettingsAnswer' },
      { id: 'stop-loss-take-profit', questionKey: 'faqStopLossTakeProfit', answerKey: 'faqStopLossTakeProfitAnswer' },
      { id: 'multiple-traders', questionKey: 'faqMultipleTraders', answerKey: 'faqMultipleTradersAnswer' },
      { id: 'ai-costs', questionKey: 'faqAICosts', answerKey: 'faqAICostsAnswer' },
    ],
  },
  {
    id: 'technical-issues',
    titleKey: 'faqCategoryTechnicalIssues',
    icon: Wrench,
    items: [
      { id: 'port-in-use', questionKey: 'faqPortInUse', answerKey: 'faqPortInUseAnswer' },
      { id: 'frontend-not-loading', questionKey: 'faqFrontendNotLoading', answerKey: 'faqFrontendNotLoadingAnswer' },
      { id: 'database-locked', questionKey: 'faqDatabaseLocked', answerKey: 'faqDatabaseLockedAnswer' },
      { id: 'ta-lib-not-found', questionKey: 'faqTALibNotFound', answerKey: 'faqTALibNotFoundAnswer' },
      { id: 'ai-api-timeout', questionKey: 'faqAIAPITimeout', answerKey: 'faqAIAPITimeoutAnswer' },
      { id: 'binance-position-mode', questionKey: 'faqBinancePositionMode', answerKey: 'faqBinancePositionModeAnswer' },
      { id: 'balance-shows-zero', questionKey: 'faqBalanceShowsZero', answerKey: 'faqBalanceShowsZeroAnswer' },
      { id: 'docker-pull-failed', questionKey: 'faqDockerPullFailed', answerKey: 'faqDockerPullFailedAnswer' },
    ],
  },
  {
    id: 'security',
    titleKey: 'faqCategorySecurity',
    icon: Shield,
    items: [
      { id: 'api-key-storage', questionKey: 'faqAPIKeyStorage', answerKey: 'faqAPIKeyStorageAnswer' },
      { id: 'encryption-details', questionKey: 'faqEncryptionDetails', answerKey: 'faqEncryptionDetailsAnswer' },
      { id: 'security-best-practices', questionKey: 'faqSecurityBestPractices', answerKey: 'faqSecurityBestPracticesAnswer' },
      { id: 'can-nofx-steal-funds', questionKey: 'faqCanNOFXStealFunds', answerKey: 'faqCanNOFXStealFundsAnswer' },
    ],
  },
  {
    id: 'features',
    titleKey: 'faqCategoryFeatures',
    icon: Star,
    items: [
      { id: 'strategy-studio', questionKey: 'faqStrategyStudio', answerKey: 'faqStrategyStudioAnswer' },
      { id: 'competition-mode', questionKey: 'faqCompetitionMode', answerKey: 'faqCompetitionModeAnswer' },
      { id: 'chain-of-thought', questionKey: 'faqChainOfThought', answerKey: 'faqChainOfThoughtAnswer' },
    ],
  },
  {
    id: 'ai-models',
    titleKey: 'faqCategoryAIModels',
    icon: Cpu,
    items: [
      { id: 'which-ai-model-best', questionKey: 'faqWhichAIModelBest', answerKey: 'faqWhichAIModelBestAnswer' },
      { id: 'custom-ai-api', questionKey: 'faqCustomAIAPI', answerKey: 'faqCustomAIAPIAnswer' },
      { id: 'ai-hallucinations', questionKey: 'faqAIHallucinations', answerKey: 'faqAIHallucinationsAnswer' },
      { id: 'compare-ai-models', questionKey: 'faqCompareAIModels', answerKey: 'faqCompareAIModelsAnswer' },
    ],
  },
  {
    id: 'contributing',
    titleKey: 'faqCategoryContributing',
    icon: Heart,
    items: [
      { id: 'how-to-contribute', questionKey: 'faqHowToContribute', answerKey: 'faqHowToContributeAnswer' },
      { id: 'github-projects-tasks', questionKey: 'faqPRGuidelines', answerKey: 'faqPRGuidelinesAnswer' },
      { id: 'contribute-pr-guidelines', questionKey: 'faqPRGuidelines', answerKey: 'faqPRGuidelinesAnswer' },
      { id: 'bounty-program', questionKey: 'faqBountyProgram', answerKey: 'faqBountyProgramAnswer' },
      { id: 'report-bugs', questionKey: 'faqReportBugs', answerKey: 'faqReportBugsAnswer' },
    ],
  },
]
