import { useEffect, useState } from "react";
import { GetBalance, GetAddress } from "../../wailsjs/go/main/App";
import { DashboardOutlined, BankOutlined, SendOutlined, SwapOutlined, ThunderboltOutlined } from "@ant-design/icons";
import { Layout, Menu, Typography, Drawer, List, message } from "antd";
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
    label: <RLink to={"faucet"}>Faucet</RLink>,
    key: "faucet",
    icon: <ThunderboltOutlined />,
  },
];

const NavBar = () => {
  const [messageApi, contextHolder] = message.useMessage();
  const [balance, setBalance] = useState("");
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
        const newBalance = await GetBalance("11111111111111111111111111111111LpoYY");
        setBalance(newBalance);
    };

    getBalance();
    const interval = setInterval(() => {
      getBalance();
    }, 500);

    return () => clearInterval(interval);
  }, []);

  return (
    <Layout.Header theme="light" style={{ background: "white" }}>
      <div
        className="logo"
        style={{ float: "left", padding: "1%" }}
      >
        <img src={logo} style={{ width: "50px" }} />
      </div>
      {/* compute to string represenation */}
      <div style={{ float: "right" }}>
        <Link strong onClick={showDrawer}>{balance} TKN</Link>
      </div>
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
      <List
        bordered
        dataSource={[{"Balance":balance, "Symbol": "TKN"}]}
        renderItem={(item) => (
          <List.Item>
            <Text>{item.Balance} {item.Symbol}</Text>
          </List.Item>
        )}
      />
    </Drawer>
    </Layout.Header>
  );
};

export default NavBar;
