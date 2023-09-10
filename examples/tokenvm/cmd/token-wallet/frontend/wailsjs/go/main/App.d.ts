// Cynhyrchwyd y ffeil hon yn awtomatig. PEIDIWCH Â MODIWL
// This file is automatically generated. DO NOT EDIT
import {main} from '../models';

export function CreateAsset(arg1:string,arg2:string,arg3:string):Promise<void>;

export function GetAccountStats():Promise<Array<main.GenericInfo>>;

export function GetAddress():Promise<string>;

export function GetBalance():Promise<Array<string>>;

export function GetChainID():Promise<string>;

export function GetLatestBlocks():Promise<Array<main.BlockInfo>>;

export function GetMyAssets():Promise<Array<main.AssetInfo>>;

export function GetTransactionStats():Promise<Array<main.GenericInfo>>;

export function GetTransactions():Promise<Array<main.TransactionInfo>>;

export function GetUnitPrices():Promise<Array<main.GenericInfo>>;
