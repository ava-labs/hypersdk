// Cynhyrchwyd y ffeil hon yn awtomatig. PEIDIWCH Â MODIWL
// This file is automatically generated. DO NOT EDIT
import {backend} from '../models';

export function AddAddressBook(arg1:string,arg2:string):Promise<void>;

export function AddAsset(arg1:string):Promise<void>;

export function CloseOrder(arg1:string,arg2:string):Promise<void>;

export function CreateAsset(arg1:string,arg2:string,arg3:string):Promise<void>;

export function CreateOrder(arg1:string,arg2:string,arg3:string,arg4:string,arg5:string):Promise<void>;

export function FillOrder(arg1:string,arg2:string,arg3:string,arg4:string,arg5:string,arg6:string):Promise<void>;

export function GetAccountStats():Promise<Array<backend.GenericInfo>>;

export function GetAddress():Promise<string>;

export function GetAddressBook():Promise<Array<backend.AddressInfo>>;

export function GetAllAssets():Promise<Array<backend.AssetInfo>>;

export function GetBalance():Promise<Array<backend.BalanceInfo>>;

export function GetChainID():Promise<string>;

export function GetFaucetSolutions():Promise<backend.FaucetSolutions>;

export function GetFeed():Promise<Array<backend.FeedObject>>;

export function GetFeedInfo():Promise<backend.FeedInfo>;

export function GetLatestBlocks():Promise<Array<backend.BlockInfo>>;

export function GetMyAssets():Promise<Array<backend.AssetInfo>>;

export function GetMyOrders():Promise<Array<backend.Order>>;

export function GetOrders(arg1:string):Promise<Array<backend.Order>>;

export function GetTransactionStats():Promise<Array<backend.GenericInfo>>;

export function GetTransactions():Promise<backend.Transactions>;

export function GetUnitPrices():Promise<Array<backend.GenericInfo>>;

export function Message(arg1:string,arg2:string):Promise<void>;

export function MintAsset(arg1:string,arg2:string,arg3:string):Promise<void>;

export function StartFaucetSearch():Promise<backend.FaucetSearchInfo>;

export function Transfer(arg1:string,arg2:string,arg3:string,arg4:string):Promise<void>;
