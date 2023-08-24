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
      "Repositories",
      "g1",
      null,
      [
        getItem(
          <Link to={"repositories/public"}>View all repositories</Link>,
          "1"
        ),
      ],
      "group"
    ),
    getItem(
      "Gists",
      "g2",
      null,
      [getItem(<Link to={"gists/public"}>View all gists</Link>, "3")],
      "group"
    ),
  ]),
  getItem("Private Actions", "sub2", <LockOutlined />, [
    getItem(
      "Repositories",
      "g3",
      null,
      [
        getItem(
          <Link to={"repositories/private"}>View my repositories</Link>,
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
        getItem(<Link to={"gists/private"}>View my gists</Link>, "6"),
        getItem(<Link to={"gist/new"}>Create new gist</Link>, "7"),
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
