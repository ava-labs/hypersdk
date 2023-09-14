import { useEffect, useState } from "react";
import { GetBalance, GetTransactions, GetAddress } from "../../wailsjs/go/main/App";
import { WalletTwoTone, DashboardOutlined, BankOutlined, SendOutlined, SwapOutlined, GoldOutlined, CheckCircleTwoTone, CloseCircleTwoTone } from "@ant-design/icons";
import { App, Layout, Menu, Typography, Drawer, List, Divider } from "antd";
const { Text, Title, Link } = Typography;
import { Link as RLink } from "react-router-dom";
import logo from "../assets/images/logo-universal.jpeg";

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
];

const NavBar = () => {
  const { message } = App.useApp();
  const [balance, setBalance] = useState([]);
  const [nativeBalance, setNativeBalance] = useState({});
  const [transactions, setTransactions] = useState([]);
  const [address, setAddress] = useState("");
  const [open, setOpen] = useState(false);

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
            break
          }
        }
        setBalance(newBalance);
    };

    const getTransactions = async () => {
        const txs = await GetTransactions();
        if (txs.Alerts !== null) {
          for (var Alert of txs.Alerts) {
            message.open({
              icon: <WalletTwoTone />, type: Alert.Type, content: Alert.Content,
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
      <div
        className="logo"
        style={{ float: "left", padding: "1%" }}
      >
        <img src={logo} style={{ width: "50px" }} />
      </div>
      {balance.length > 0 &&
        <div style={{ float: "right" }}>
          <Link strong onClick={showDrawer}>{nativeBalance.Str}</Link>
        </div>
      }
      <Menu
        defaultSelectedKeys={["explorer"]}
        mode="horizontal"
        items={items}
        style={{
          position: "relative",
        }}
      />
    <Drawer title={<Text copyable>{address}</Text>} size={"large"} placement="right" onClose={onClose} open={open}>
      {/* use a real data source */}
      <Divider orientation="center">
        Tokens
      </Divider>
      <List
        bordered
        dataSource={balance}
        renderItem={(item) => (
          <List.Item>
            <Text>{item.Str}</Text>
          </List.Item>
        )}
      />
      <Divider orientation="center">
        Transactions
      </Divider>
      <List
        bordered
        dataSource={transactions}
        renderItem={(item) => (
          <List.Item>
            <div>
              <Text strong>{item.ID} </Text>
              {!item.Success &&
                <CloseCircleTwoTone twoToneColor="#eb2f96" />
              }
              {item.Success &&
                <CheckCircleTwoTone twoToneColor="#52c41a" />
              }
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
