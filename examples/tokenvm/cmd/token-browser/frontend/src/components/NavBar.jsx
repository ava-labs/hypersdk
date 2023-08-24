import { LockOutlined, UnlockOutlined } from "@ant-design/icons";
import { Layout, Menu } from "antd";
import { Link } from "react-router-dom";
import logo from "../assets/images/logo-universal.png";

function getItem(label, key, icon, children, type) {
  return {
    key,
    icon,
    children,
    label,
    type,
  };
}
const items = [
  getItem("Public Actions", "sub1", <UnlockOutlined />, [
    getItem(
      "Keys",
      "g1",
      null,
      [
        getItem(
          <Link to={"keys"}>Keys</Link>,
          "1"
        ),
      ],
      "group"
    ),
    getItem(
      "Keys2",
      "g2",
      null,
      [getItem(<Link to={"keys"}>Keys 2</Link>, "3")],
      "group"
    ),
  ]),
  getItem("Private Actions", "sub2", <LockOutlined />, [
    getItem(
      "Chains",
      "g3",
      null,
      [
        getItem(
          <Link to={"chains"}>chains</Link>,
          "5"
        ),
      ],
      "group"
    ),
    getItem(
      "Gists",
      "g4",
      null,
      [
        getItem(<Link to={"chains"}>Chains 2</Link>, "6"),
        getItem(<Link to={"chains"}>Chains 3</Link>, "7"),
      ],
      "group"
    ),
  ]),
];

const NavBar = () => {
  return (
    <Layout.Header theme="light" style={{ background: "white" }}>
      <div
        className="logo"
        style={{
          float: "left",
          marginRight: "200px",
          padding: "1%",
        }}
      >
        <Link to="/">
          <img src={logo} style={{ width: "50px" }} />
        </Link>
      </div>
      <Menu
        defaultSelectedKeys={["1"]}
        mode="horizontal"
        items={items}
        style={{
          position: "relative",
        }}
      />
    </Layout.Header>
  );
};

export default NavBar;
