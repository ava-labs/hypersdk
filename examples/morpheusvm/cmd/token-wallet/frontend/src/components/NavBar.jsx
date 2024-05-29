import { useEffect, useState } from "react";
import {
  GetBalance,
  GetTransactions,
  GetAddress,
} from "../../wailsjs/go/main/App";
import {
  WalletTwoTone,
  DashboardOutlined,
  BankOutlined,
  SendOutlined,
  SwapOutlined,
  GoldOutlined,
  CheckCircleTwoTone,
  CloseCircleTwoTone,
  ContainerOutlined,
} from "@ant-design/icons";
import { App, Layout, Menu, Typography, Drawer, List, Divider } from "antd";
const { Text, Link } = Typography;
import { useLocation, Link as RLink } from "react-router-dom";
import logo from "../assets/images/logo-universal.png";

const NavBar = () => {
  const location = useLocation();
  const { message } = App.useApp();
  const [balance, setBalance] = useState([]);
  const [nativeBalance, setNativeBalance] = useState({});
  const [transactions, setTransactions] = useState([]);
  const [address, setAddress] = useState("");
  const [open, setOpen] = useState(false);

  const items = [
    {
      label: <RLink to={"explorer"}>Explorer</RLink>,
      key: "explorer",
      icon: <DashboardOutlined />,
    },
    {
      label: <RLink to={"faucet"}>Faucet</RLink>,
      key: "faucet",
      icon: <GoldOutlined />,
    },
    {
      label: <RLink to={"mint"}>Mint</RLink>,
      key: "mint",
      icon: <BankOutlined />,
    },
    {
      label: <RLink to={"transfer"}>Transfer</RLink>,
      key: "transfer",
      icon: <SendOutlined />,
    },
    {
      label: <RLink to={"trade"}>Trade</RLink>,
      key: "trade",
      icon: <SwapOutlined />,
    },
    {
      label: <RLink to={"feed"}>Feed</RLink>,
      key: "feed",
      icon: <ContainerOutlined />,
    },
  ];

  const showDrawer = () => {
    setOpen(true);
  };

  const onClose = () => {
    setOpen(false);
  };

  useEffect(() => {
    const getAddress = async () => {
      const address = await GetAddress();
      setAddress(address);
    };
    getAddress();

    const getBalance = async () => {
      const newBalance = await GetBalance();
      for (var bal of newBalance) {
        if (bal.ID == "11111111111111111111111111111111LpoYY") {
          setNativeBalance(bal);
          {
            /* TODO: switch to using context */
          }
          window.HasBalance = bal.Has;
          break;
        }
      }
      setBalance(newBalance);
    };

    const getTransactions = async () => {
      const txs = await GetTransactions();
      if (txs.Alerts !== null) {
        for (var Alert of txs.Alerts) {
          message.open({
            icon: <WalletTwoTone />,
            type: Alert.Type,
            content: Alert.Content,
          });
        }
      }
      setTransactions(txs.TxInfos);
    };

    getBalance();
    getTransactions();
    const interval = setInterval(() => {
      getBalance();
      getTransactions();
    }, 500);

    return () => clearInterval(interval);
  }, []);

  return (
    <>
      <Layout.Header theme="light" style={{ background: "white" }}>
        <div className="logo" style={{ float: "left", margin: "8px" }}>
          <img src={logo} style={{ width: "50px" }} />
        </div>
        {balance.length > 0 && (
          <div style={{ float: "right" }}>
            <Link strong onClick={showDrawer}>
              {nativeBalance.Str}
            </Link>
          </div>
        )}
        <Menu
          mode="horizontal"
          items={items}
          style={{
            position: "relative",
          }}
          selectedKeys={
            location.pathname.length > 1
              ? [location.pathname.slice(1)]
              : ["explorer"]
          }
        />
        <Drawer
          title={<Text copyable>{address}</Text>}
          size={"large"}
          placement="right"
          onClose={onClose}
          open={open}>
          {/* use a real data source */}
          <Divider orientation="center">Tokens</Divider>
          <List
            bordered
            dataSource={balance}
            renderItem={(item) => (
              <List.Item>
                <Text>{item.Str}</Text>
              </List.Item>
            )}
          />
          <Divider orientation="center">Transactions</Divider>
          <List
            bordered
            dataSource={transactions}
            renderItem={(item) => (
              <List.Item>
                <div>
                  <Text strong>{item.ID} </Text>
                  {!item.Success && (
                    <CloseCircleTwoTone twoToneColor="#eb2f96" />
                  )}
                  {item.Success && (
                    <CheckCircleTwoTone twoToneColor="#52c41a" />
                  )}
                </div>
                <Text strong>Type:</Text> {item.Type}
                <br />
                <Text strong>Timestamp:</Text> {item.Timestamp}
                <br />
                <Text strong>Units:</Text> {item.Units}
                <br />
                <Text strong>Size:</Text> {item.Size}
                <br />
                <Text strong>Summary:</Text> {item.Summary}
                <br />
                <Text strong>Fee:</Text> {item.Fee}
                <br />
                <Text strong>Actor:</Text> <Text copyable>{item.Actor}</Text>
              </List.Item>
            )}
          />
        </Drawer>
      </Layout.Header>
    </>
  );
};

export default NavBar;
