export const appEdition = __LINGUAQUEST_APP_EDITION__;
export const isMiniProgramEdition = appEdition === "MINI_PROGRAM";
export const isCommercialEdition = __LINGUAQUEST_COMMERCIAL_EDITION__;
export const isOpenSourceEdition = !isCommercialEdition;
